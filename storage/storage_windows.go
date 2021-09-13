// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/google/logger"
	glstor "github.com/google/glazier/go/storage"
)

const (
	// The GPT GUID to use when created = 'Basic Data'
	// https://docs.microsoft.com/en-us/powershell/module/storage/new-partition?view=win10-ps
	basic = `{ebd0a0a2-b9e5-4433-87c0-68b6b72699c7}`
	// Windows will not format FAT32 paritions larger than 32 GB. These
	// Allow us to handle drives >32GB on any platform.
	maxWinFAT32Size = 30648342272     // Maximum partition size for FAT32 on Windows.
	maxPartSize     = maxWinFAT32Size // Maximum FAT32 size on Windows.
	partAlignment   = 4194304         // 4MiB alignment for read performance.
)

var (
	// Wrapped errors for testing.
	errNoOutput              = errors.New(`output was empty`)
	errDetectVol             = errors.New(`volume detection error`)
	errDriveLetter           = errors.New(`drive letter error`)
	errOSResource            = errors.New(`resource unavailable`)
	errMount                 = errors.New(`partition mount error`)
	errClearDisk             = errors.New(`disk clearing error`)
	errSetDisk               = errors.New(`property setting error`)
	errPowershell            = errors.New("powershell error")
	errUnsupportedFileSystem = errors.New("unsupported filesystem")

	// Regex for powershell error handling.
	regExPSGetVolErr           = regexp.MustCompile(`Get-Volume[\s\S]+`)
	regExPSRemoveAccessPathErr = regexp.MustCompile(`Remove-PartitionAccessPath[\s\S]+`)
	regExPSSetPartErr          = regexp.MustCompile(`Set-Partition[\s\S]+`)

	// Dependency injection for testing.
	availableDrives = freeDrives
	getVolumeCmd    = powershell
	powershellCmd   = powershell
	setPartitionCmd = powershell
	fnConnect       = windowsConnect
	fnGetDisks      = windowsGetDisks
	fnGetPartitions = windowsGetPartitions
	fnGetVolumes    = windowsGetVolumes
	fnInitialize    = windowsInitialize
	fnFormat        = windowsFormat
	fnPartition     = windowsPartition
	fnWipe          = windowsWipe
)

// Search performs a device search based on the provided parameters and returns
// a slice of storage devices that meet the criteria. Parameters are not
// mandatory. For example, if no deviceID is passed, all deviceID's are
// considered for the search.
func Search(deviceID string, minSize, maxSize uint64, removableOnly bool) ([]*Device, error) {
	svc, err := fnConnect()
	if err != nil {
		return nil, err
	}
	defer svc.Close()

	query := ""
	if deviceID != "" {
		query = fmt.Sprintf("WHERE Number=%s", deviceID)
	}
	disks, err := fnGetDisks(query, svc)
	if err != nil {
		return nil, fmt.Errorf("svc.GetDisks(%s): %w", query, err)
	}
	defer disks.Close()

	found := []*Device{}
	for _, d := range disks.Disks {
		// Build Device
		device := &Device{
			id: fmt.Sprint(d.Number),
			// TODO(@itsmattl): make helper constants for bus types
			removable: d.BusType == 7,
			size:      d.Size,
			make:      strings.TrimSpace(d.Manufacturer),
			model:     strings.TrimSpace(d.Model),
			partStyle: glstor.PartitionStyle(d.PartitionStyle),
		}
		if removableOnly && !device.removable {
			continue
		}
		if device.size < minSize {
			continue
		}
		if device.size > maxSize && maxSize != 0 {
			continue
		}
		// Add Partition Information and assign drive letters if missing.
		addLetter := true
		if err := device.DetectPartitions(addLetter); err != nil {
			return nil, fmt.Errorf("DetectPartitions() for %q returned %v", device.Identifier(), err)
		}
		found = append(found, device)
	}
	return found, nil
}

// New takes a raw device ID and returns a pointer to a Device
// that describes the storage media and its contents or an error.
func New(deviceID string) (*Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("empty device ID: %w", errInput)
	}
	devices, err := Search(deviceID, 0, 0, false)
	if err != nil {
		return nil, fmt.Errorf("search(%q, 0, 0, false) returned %v: %w", deviceID, err, errDisk)
	}
	// New is expected to only ever return one device, multiples are unexpected.
	if len(devices) > 1 {
		var list string
		for _, d := range devices {
			list = list + fmt.Sprintf("%q ", d.id)
		}
		return nil, fmt.Errorf("New(%q) returned more than one device (%q): %w", deviceID, list, errNoMatch)
	}
	return devices[0], nil
}

// DetectPartitions updates a device with information for known partitions on
// Windows.
func (device *Device) DetectPartitions(assignLetter bool) error {
	if device.id == "" {
		return fmt.Errorf("device ID was empty: %w", errInput)
	}

	svc, err := fnConnect()
	if err != nil {
		return err
	}
	defer svc.Close()

	parts, err := fnGetPartitions(fmt.Sprintf("WHERE DiskNumber=%s", device.id), svc)
	if err != nil {
		return err
	}
	defer parts.Close()

	// Process available partitions and add them to the drive.
	partitions := []Partition{}
	for _, part := range parts.Partitions {
		if len(part.AccessPaths) < 1 {
			logger.Warningf("no access path for partition %v", part)
			continue
		}
		path := ""
		for _, p := range part.AccessPaths {
			if strings.HasPrefix(p, `\\`) {
				path = p
			}
		}
		p := Partition{
			disk:  fmt.Sprint(part.DiskNumber),
			id:    fmt.Sprint(part.PartitionNumber),
			path:  path,
			mount: part.DriveLetter,
			size:  part.Size,
		}

		vol, err := fnGetVolumes(path, svc)
		if err != nil {
			return err
		}

		fs, ok := fileSystems[vol.Volumes[0].FileSystem]
		if !ok {
			fs = UnknownFS
		}
		p.fileSystem = fs
		vol.Close()

		// Assign drive letters for compatible partitions to allow reading of the
		// device label. Drive letters are only assigned if they are already
		// missing and the binary is running with elevated privileges. Drive letter
		// assignments are gated on having only one partition on disk to limit
		// unnecessary drive letter assignments. Errors here are not fatal but
		// are logged.
		if len(parts.Partitions) == 1 && assignLetter {
			if err := p.Mount(""); err != nil {
				logger.V(2).Infoln(err)
			}
		}

		// Add label information for volumes that are mounted. An inability to read
		// a label is not fatal but is logged.
		if err := p.addLabel(); err != nil {
			logger.V(2).Infoln(err)
		}

		// Append the complete partition.
		partitions = append(partitions, p)
	}
	device.partitions = partitions

	return nil
}

// Wipe removes all filesystem and partition table signatures from the
// device. If the device is not already GPT style, it is converted
// to GPT as part of the wipe.
func (device *Device) Wipe() error {
	if device.id == "" {
		return errInput
	}

	svc, err := fnConnect()
	if err != nil {
		return err
	}
	defer svc.Close()

	disks, err := fnGetDisks(fmt.Sprintf("WHERE Number=%s", device.id), svc)
	if err != nil {
		return err
	}
	defer disks.Close()

	if err := fnWipe(&disks.Disks[0]); err != nil {
		return err
	}

	if err := fnInitialize(&disks.Disks[0]); err != nil {
		return err
	}

	// Update the disk to reflect the cleared partitions.
	device.partitions = []Partition{}
	// Update the disk to reflect the new partition style.
	device.partStyle = glstor.GptStyle

	return nil
}

// Partition partitions a GPT-style device with a single basic data partition.
func (device *Device) Partition(label string) error {
	size := device.size
	if device.size >= maxPartSize {
		size = maxPartSize
	}
	return device.PartitionWithOptions(label, glstor.GptTypes.BasicData, size)
}

// PartitionWithOptions partitions a GPT-style device with a single partition.
// It is assumed that the partition table is already empty. If it is not,
// a corresponding PowerShell error will be observed. The maximum amount
// of available space is used for the partition span. On Windows, this is
// limited to 32 GB for FAT32.
func (device *Device) PartitionWithOptions(label string, gType glstor.GptType, size uint64) error {
	if device.id == "" {
		return errInput
	}
	if len(device.partitions) != 0 {
		return fmt.Errorf("partition table not empty: %w", errDisk)
	}

	// Adjust arguments for larger removable drives.
	useMax := true
	if size > 0 {
		if size < device.Size() {
			useMax = false
		} else {
			size = 0 // let windows api use maximum available size
			logger.V(1).Infof("shrinking partition size to fit on disk")
		}
	}

	svc, err := fnConnect()
	if err != nil {
		return err
	}
	defer svc.Close()

	disks, err := fnGetDisks(fmt.Sprintf("WHERE Number=%s", device.id), svc)
	if err != nil {
		return err
	}
	defer disks.Close()

	if err := fnPartition(&disks.Disks[0], size, useMax, gType); err != nil {
		return fmt.Errorf("%w: %v", err, errPartition)
	}

	// Update the disk with the new partition information.
	device.partStyle = glstor.GptStyle
	if err := device.DetectPartitions(false); err != nil {
		return fmt.Errorf("DetectPartitions() for %q returned %v: %w", device.Identifier(), err, errDisk)
	}
	return nil
}

// Dismount removes the drive letter assignments for a device on Windows
// to limit accidental writes to the volumes. It is typically used with a
// flag when safety after writes to disk are desired.
func (device *Device) Dismount() error {
	if len(device.partitions) < 1 {
		return nil
	}
	for _, part := range device.partitions {
		// Skip partitions with no drive letter.
		if part.mount == "" {
			continue
		}
		// e.g.: Remove-PartitionAccessPath -Disknumber 1 -PartitionNumber 1 -AccessPath "D:\"
		psBlock := fmt.Sprintf(`Remove-PartitionAccessPath -DiskNumber %s -PartitionNumber %s -AccessPath '%s:\'`, device.id, part.id, part.mount)
		out, err := powershellCmd(psBlock)
		if err != nil {
			return fmt.Errorf("%v: %w", err, errPowershell)
		}
		if regExPSRemoveAccessPathErr.Match(out) {
			return fmt.Errorf("%v: %w", out, errDisk)
		}
		// Clear the drive letter from the partition
		part.mount = ""
	}
	return nil
}

// Eject does nothing on Windows. It is maintained to keep the implementation
// consistent between platforms.
func (device *Device) Eject() error {
	return nil
}

// Mount reads the current drive letter assignment for the partition.
// If it is empty and the file system is readable by Windows, one is assigned.
// IMPORTANT: This operation is expensive. It takes between 5 and 15 seconds
// to run. The caller should take care to only invoke this function if the
// label is needed.
func (part *Partition) Mount(letter string) error {
	// Skip partitions that already have drive letters.
	if part.mount != "" {
		return nil
	}
	// Skip partitions that cannot be assigned a drive letter.
	if part.fileSystem == UnknownFS {
		return nil
	}
	// TODO(b/130833261) We need to check for Admin here and skip if we are not.

	// Assign partitions to everything else.
	available, err := freeDrive(letter)
	if err != nil {
		return fmt.Errorf("availableDrive(%s) returned %v: %w", letter, err, errInput)
	}
	lBlock := fmt.Sprintf(`Set-Partition -DiskNumber %s -PartitionNumber %s -NewDriveLetter %s`, part.disk, part.id, available)
	lOut, err := setPartitionCmd(lBlock)
	if err != nil {
		return fmt.Errorf("powershell returned %v: %w", err, errPowershell)
	}
	if regExPSSetPartErr.Match(lOut) {
		return fmt.Errorf("powershell returned %v: %w", lOut, errDriveLetter)
	}

	// Update the partition with its drive letter and return successfully.
	part.mount = available

	return nil
}

// addLabel reads the label for a mounted partition with a drive letter and
// updates the partition with the label information. If label is invoked
// on a partition that does not have a drive letter, it silently returns.
func (part *Partition) addLabel() error {
	if part.mount == "" {
		return nil
	}

	// e.g.: Get-Volume D | select FileSystemLabel | ConvertTo-Json
	vBlock := fmt.Sprintf(`Get-Volume %s | select FileSystemLabel | ConvertTo-Json`, part.mount)
	vOut, err := getVolumeCmd(vBlock)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errPowershell)
	}
	if regExPSGetVolErr.Match(vOut) {
		return fmt.Errorf("%v: %w", vOut, errDetectVol)
	}
	if len(vOut) == 0 {
		return fmt.Errorf("unable to enumerate properties for %q: %w", part.mount, errNoOutput)
	}
	vol := &struct {
		Label string `json:"FileSystemLabel"`
	}{}
	if err := json.Unmarshal(vOut, vol); err != nil {
		return fmt.Errorf("json.Unmarshal returned %v: %w", err, errUnmarshal)
	}

	// Update the partition and return.
	part.label = vol.Label
	return nil
}

// freeDrives returns a string slice of available drive letters
func freeDrives() []string {
	var openeable []string
	for _, drive := range "FGHIJKLMNOPQRSTUVWXYZ" {
		d := string(drive)
		h, err := os.Open(d + `:\`)
		if err != nil {
			openeable = append(openeable, d)
			h.Close()
		}
	}
	return openeable
}

// freeDrive returns an unassigned Windows drive letter. It accepts a requested
// drive letter as a parameter. If the requested drive letter is not available
// an error is returned.
func freeDrive(requested string) (string, error) {
	available := availableDrives()
	if len(available) == 0 {
		return "", fmt.Errorf("no free drives available: %w", errOSResource)
	}
	// Check if the optional requested drive letter is available.
	if requested != "" {
		for _, l := range available {
			if l == requested {
				return l, nil
			}
		}
		return "", fmt.Errorf("requested drive %q not found: %w", requested, errDriveLetter)
	}
	return available[0], nil
}

// Format formats the corresponding partition as FAT32.
func (part *Partition) Format(label string) error {
	return part.FormatWithOptions(label, FAT32, true)
}

// FormatWithOptions formats the corresponding partition with additional options.
func (part *Partition) FormatWithOptions(label string, filesystem FileSystem, mount bool) error {
	if part.path == "" {
		return errInput
	}

	svc, err := fnConnect()
	if err != nil {
		return err
	}
	defer svc.Close()

	vol, err := fnGetVolumes(part.path, svc)
	if err != nil {
		return err
	}
	defer vol.Close()

	if err := fnFormat(filesystem, label, &vol.Volumes[0]); err != nil {
		return err
	}

	if mount {
		// Mount the partition that was just formatted.
		if err := part.Mount(""); err != nil {
			return fmt.Errorf("Mount() returned %v: %w", err, errMount)
		}
	}
	return nil
}

type iDisk interface {
	Close()
	Clear(removeData, removeOEM, zeroDisk bool) (glstor.ExtendedStatus, error)
	ConvertStyle(style glstor.PartitionStyle) (glstor.ExtendedStatus, error)
	CreatePartition(size uint64, useMaximumSize bool, offset uint64, alignment int, driveLetter string, assignDriveLetter bool,
		mbrType *glstor.MbrType, gptType *glstor.GptType, hidden, active bool) (glstor.Partition, glstor.ExtendedStatus, error)
	Initialize(ps glstor.PartitionStyle) (glstor.ExtendedStatus, error)
}

type iService interface {
	Close()
	GetDisks(string) (glstor.DiskSet, error)
	GetPartitions(string) (glstor.PartitionSet, error)
	GetVolumes(string) (glstor.VolumeSet, error)
}

type iVolume interface {
	FormatFAT32(label string, allocationUnitSize int32, full, force bool) (glstor.Volume, glstor.ExtendedStatus, error)
	FormatNTFS(label string, allocationUnitSize int32, full, force, compress, shortFileNameSupport, useLargeFRS, disableHeatGathering bool) (glstor.Volume, glstor.ExtendedStatus, error)
}

func windowsConnect() (iService, error) {
	svc, err := glstor.Connect()
	return &svc, err
}

func windowsFormat(fs FileSystem, label string, vol iVolume) error {
	var v glstor.Volume
	var err error
	switch fs {
	case FAT32:
		v, _, err = vol.FormatFAT32(label, 0, false, true)
	case NTFS:
		v, _, err = vol.FormatNTFS(label, 0, false, true, true, true, true, false)
	default:
		return fmt.Errorf("%w: %v", errUnsupportedFileSystem, fs)
	}
	if err != nil {
		return err
	}
	v.Close()
	return nil
}

func windowsGetDisks(query string, svc iService) (glstor.DiskSet, error) {
	ds, err := svc.GetDisks(query)
	if err != nil {
		return glstor.DiskSet{}, err
	}
	if len(ds.Disks) < 1 {
		return glstor.DiskSet{}, errDetectDisk
	}
	return ds, nil
}

func windowsGetPartitions(query string, svc iService) (glstor.PartitionSet, error) {
	ps, err := svc.GetPartitions(query)
	if err != nil {
		return glstor.PartitionSet{}, err
	}
	return ps, nil
}

func windowsGetVolumes(path string, svc iService) (glstor.VolumeSet, error) {
	path = strings.ReplaceAll(path, `\`, `\\`) // escape the slashes used in volume paths
	vol, err := svc.GetVolumes(fmt.Sprintf("WHERE Path='%s'", path))
	if err != nil {
		return glstor.VolumeSet{}, err
	}
	if len(vol.Volumes) < 1 {
		return glstor.VolumeSet{}, errDetectVol
	}
	return vol, nil
}

func windowsInitialize(disk iDisk) error {
	_, err := disk.ConvertStyle(glstor.GptStyle)
	return err
}

func windowsPartition(disk iDisk, size uint64, max bool, gptType glstor.GptType) error {
	var gt *glstor.GptType
	switch string(gptType) {
	case string(glstor.GptTypes.SystemPartition):
		gt = &glstor.GptTypes.SystemPartition
	case string(glstor.GptTypes.BasicData):
		gt = &glstor.GptTypes.BasicData
	}

	part, _, err := disk.CreatePartition(size, max, 0, partAlignment, "", false, nil, gt, false, false)
	part.Close()
	return err
}

func windowsWipe(disk iDisk) error {
	_, err := disk.Clear(true, true, false)
	return err
}

// powershell represents the OS command used to run a powershell cmdlet on
// Windows. It accepts a string representing a command block. The raw output is
// provided to the caller for handling. An error is returned if powershell.exe
// returns it, but if a powershell cmdlet throws an error this does not cause
// powershell.exe to return an error. Thus, output issues and cmdlet errors
// should be handled by the caller.
func powershell(psBlock string) ([]byte, error) {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", psBlock).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("powershell.exe", "-NoProfile", "-Command", %s) command returned: %q: %v`, psBlock, out, err)
	}
	return out, nil
}
