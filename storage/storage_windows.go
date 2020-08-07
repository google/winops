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

// +build windows

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/logger"
)

const (
	// The GPT GUID to use when created = 'Basic Data'
	// https://docs.microsoft.com/en-us/powershell/module/storage/new-partition?view=win10-ps
	basic = `{ebd0a0a2-b9e5-4433-87c0-68b6b72699c7}`
	// Windows will not format FAT32 paritions larger than 32 GB. These
	// Allow us to handle drives >32GB on any platform.
	maxWinFAT32Size = 30648342272                  // Maximum partition size for FAT32 on Windows.
	partOffset      = 4294656                      // 4MiB alignment for read performance.
	maxPartSize     = maxWinFAT32Size - partOffset // Maximum FAT32 size on Windows.

)

var (
	// Wrapped errors for testing.
	errDetectPart  = errors.New(`partition detection error`)
	errNoOutput    = errors.New(`output was empty`)
	errDetectVol   = errors.New(`volume detection error`)
	errDriveLetter = errors.New(`drive letter error`)
	errOSResource  = errors.New(`resource unavailable`)
	errMount       = errors.New(`partition mount error`)
	errClearDisk   = errors.New(`disk clearing error`)
	errSetDisk     = errors.New(`property setting error`)
	errPowershell  = errors.New("powershell error")

	// Regex for powershell error handling.
	regExPSClearDiskErr        = regexp.MustCompile(`Clear-Disk[\s\S]+`)
	regExPSFormatVolErr        = regexp.MustCompile(`Format-Volume[\s\S]+`)
	regExPSGetVolErr           = regexp.MustCompile(`Get-Volume[\s\S]+`)
	regExPSGetDiskErr          = regexp.MustCompile(`Get-Disk[\s\S]+`)
	regExPSGetPartErr          = regexp.MustCompile(`Get-Partition[\s\S]+`)
	regExPSGetPartNoPartErr    = regexp.MustCompile(`Get-Partition : No MSFT_Partition objects found[\s\S]+`)
	regExPSNewPartErr          = regexp.MustCompile(`New-Partition[\s\S]+`)
	regExPSRemoveAccessPathErr = regexp.MustCompile(`Remove-PartitionAccessPath[\s\S]+`)
	regExPSSetDiskErr          = regexp.MustCompile(`Set-Disk[\s\S]+`)
	regExPSSetPartErr          = regexp.MustCompile(`Set-Partition[\s\S]+`)

	// Dependency injection for testing.
	availableDrives = freeDrives
	getVolumeCmd    = powershell
	powershellCmd   = powershell
	setPartitionCmd = powershell
)

// Search performs a device search based on the provided parameters and returns
// a slice of storage devices that meet the criteria. Parameters are not
// mandatory. For example, if no deviceID is passed, all deviceID's are
// considered for the search.
func Search(deviceID string, minSize, maxSize uint64, removableOnly bool) ([]*Device, error) {
	// The output of Get-Disk is wrapped in an array to ensure that a single disk
	// renders the same as multiple disks.
	dBlock := fmt.Sprintf(`ConvertTo-Json @(Get-Disk %s | Select-Object DiskNumber, PartitionStyle, BusType, Path, Size)`, deviceID)
	if deviceID == "" {
		dBlock = fmt.Sprint(`ConvertTo-Json @(Get-Disk | Select-Object DiskNumber, PartitionStyle, BusType, Path, Size, Manufacturer, Model)`)
	}
	dOut, err := powershellCmd(dBlock)
	if err != nil {
		return nil, fmt.Errorf("powershell returned %v: %w", err, errPowershell)
	}
	if regExPSGetDiskErr.Match(dOut) {
		return nil, fmt.Errorf("powershell commandlet returned %v: %w", dOut, errDisk)
	}
	if len(dOut) == 0 {
		return nil, fmt.Errorf("unable to get disk information: %w", errNoOutput)
	}
	disks := &[]struct {
		Manufacturer   string `json:"Manufacturer"`
		Model          string `json:"Model"`
		DiskNumber     int    `json:"DiskNumber"`
		PartitionStyle string `json:"PartitionStyle"`
		BusType        string `json:"BusType"`
		Size           uint64 `json:"Size"`
	}{}
	if err := json.Unmarshal(dOut, disks); err != nil {
		return nil, fmt.Errorf("json.Unmarshal returned %v: %w", err, errUnmarshal)
	}

	found := []*Device{}
	for _, d := range *disks {
		// Build Device
		device := &Device{
			id:        strconv.Itoa(d.DiskNumber),
			removable: d.BusType == "USB",
			size:      d.Size,
			make:      strings.TrimSpace(d.Manufacturer),
			model:     strings.TrimSpace(d.Model),
			partStyle: d.PartitionStyle,
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

type pGetPartitionResult struct {
	DiskNum      int    `json:"DiskNumber"`
	DriveLetter  string `json:"DriveLetter,omitempty"`
	PartitionNum int    `json:"PartitionNumber"`
	Path         string `json:"Path"`
	Size         uint64 `json:"Size"`
	Type         string `json:"Type"`
}

type pGetPartitionResults []pGetPartitionResult

// DetectPartitions updates a device with information for known partitions on
// Windows.
func (device *Device) DetectPartitions(assignLetter bool) error {
	if device.id == "" {
		return fmt.Errorf("device ID was empty: %w", errInput)
	}

	// e.g.: Get-Partition -DiskNumber 1 | select @{n='Path';e={$_.AccessPaths[1]}}, Size | ConvertTo-Json
	// Note on "Custom" or "Calculated" PowerShell properties, i.e., @{n='NAME';e={EXPRESSION}}
	// Here we leverage them to derive the unique path to the volume, e.g.:
	// "\\?\Volume{a11dd3fc-b4d2-11e9-b27d-3cf01167ff7e}\" as opposed to the
	// drive letter, e.g.: "D:\". the output of Get-Partition is wrapped in an
	// array to ensure that a single partition renders the same as multiple
	// partitions.
	pBlock := fmt.Sprintf(`ConvertTo-Json @(Get-Partition -DiskNumber %s | select @{n='Path';e={$_.AccessPaths[0]}}, Size, PartitionNumber, DriveLetter, Type, DiskNumber)`, device.id)
	pOut, err := powershellCmd(pBlock)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errPowershell)
	}
	// PowerShell throws an error if there are no partitions. In this case we
	// do not return the error and update the device with no partitions instead.
	if regExPSGetPartErr.Match(pOut) {
		if regExPSGetPartNoPartErr.Match(pOut) {
			device.partitions = []Partition{}
			return nil
		}
		return fmt.Errorf("powershell call %q returned %s: %w", pBlock, pOut, errDetectPart)
	}
	if len(pOut) == 0 {
		return fmt.Errorf("unable to enumerate partitions: %w", errNoOutput)
	}

	pGetPartition := pGetPartitionResults{}
	if err := json.Unmarshal(pOut, &pGetPartition); err != nil {
		return fmt.Errorf("json.Unmarshal returned %v for output %q : %w", err, pOut, errUnmarshal)
	}

	// Process available partitions and add them to the drive.
	partitions := []Partition{}
	for _, part := range pGetPartition {
		fs, ok := fileSystems[part.Type]
		if !ok {
			fs = UnknownFS
		}

		p := Partition{
			disk:       strconv.Itoa(part.DiskNum),
			id:         strconv.Itoa(part.PartitionNum),
			path:       part.Path,
			mount:      part.DriveLetter,
			fileSystem: fs,
			size:       part.Size,
		}
		// Assign drive letters for compatible partitions to allow reading of the
		// device label. Drive letters are only assigned if they are already
		// missing and the binary is running with elevated privileges. Drive letter
		// assignments are gated on having only one partition on disk to limit
		// unnecessary drive letter assignments. Errors here are not fatal but
		// are logged.
		if len(pGetPartition) == 1 && assignLetter {
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
	// Clear all existing partitions from the disk.
	// e.g.: Clear-Disk -Number 1 -RemoveData -RemoveOEM -Confirm:$false
	cBlock := fmt.Sprintf(`Clear-Disk -Number '%s' -RemoveData -RemoveOEM -Confirm:$false`, device.id)
	cOut, err := powershellCmd(cBlock)
	if err != nil {
		return fmt.Errorf("powershell returned %v: %w", err, errWipe)
	}
	if regExPSClearDiskErr.Match(cOut) {
		return fmt.Errorf("Clear-Disk returned %v: %w", cOut, errDisk)
	}
	// Update the disk to reflect the cleared partitions.
	device.partitions = []Partition{}

	// Set the partition style to GPT
	// e.g. Set-Disk -Number 1 -PartitionStyle GPT
	sBlock := fmt.Sprintf(`Set-Disk -Number %s -PartitionStyle GPT`, device.id)
	sOut, err := powershellCmd(sBlock)
	if err != nil {
		return fmt.Errorf("powershell returned %v: %w", err, errPowershell)
	}
	if regExPSSetDiskErr.Match(sOut) {
		return fmt.Errorf("Set-Disk returned %v: %w", sOut, errSetDisk)
	}
	//Update the disk to reflect the new partition style.
	device.partStyle = string(gpt)

	return nil
}

// Partition partitions a GPT-style device with a single FAT32 partition.
// It is assumed that the partition table is already empty. If it is not,
// a corresponding PowerShell error will be observed. The maximum amount
// of available space is used for the partition span. On Windows, this is
// limited to 32 GB for FAT32.
func (device *Device) Partition(label string) error {
	if device.id == "" {
		return errInput
	}
	if len(device.partitions) != 0 {
		return fmt.Errorf("partition table not empty: %w", errDisk)
	}
	// Adjust arguments for larger removable drives.
	size := "-UseMaximumSize"
	if device.size >= maxPartSize {
		size = fmt.Sprintf("-Size %d", maxPartSize)
	}

	// e.g.: New-Partition -DiskNumber 1 -GptType {ebd0a0a2-b9e5-4433-87c0-68b6b72699c7} -Offset 4294656 -Size 8GB
	psBlock := fmt.Sprintf(`New-Partition -DiskNumber %s -GptType '%s' -Offset %d %s`, device.id, basic, partOffset, size)
	out, err := powershellCmd(psBlock)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errPowershell)
	}
	if regExPSNewPartErr.Match(out) {
		return fmt.Errorf("%v: %w", out, errPartition)
	}

	// Update the disk with the new partition information.
	device.partStyle = string(gpt)
	addLetter := false
	if err := device.DetectPartitions(addLetter); err != nil {
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
	// TODO We need to check for Admin here and skip if we are not.

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
// TODO Parameters to allow formatting other filesystems.
func (part *Partition) Format(label string) error {
	if part.path == "" {
		return errInput
	}
	// e.g.: Format-Volume -Path '\\?\Volume{a11dd3fc-b4d2-11e9-b27d-3cf01167ff7e}\' -FileSystem FAT32
	psBlock := fmt.Sprintf(`Format-Volume -Path '%s' -FileSystem FAT32 -NewFileSystemLabel '%s'`, part.path, label)
	out, err := powershellCmd(psBlock)
	if err != nil {
		return fmt.Errorf("powershell returned %v: %w", err, errPowershell)
	}
	if regExPSFormatVolErr.Match(out) {
		return fmt.Errorf("powershell returned %v: %w", out, errFormat)
	}
	// Mount the partition that was just formatted.
	if err := part.Mount(""); err != nil {
		return fmt.Errorf("Mount() returned %v: %w", err, errMount)
	}
	return nil
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
