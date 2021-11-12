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

//go:build darwin
// +build darwin

package storage

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/groob/plist"
	glstor "github.com/google/glazier/go/storage"
)

const (
	// Space in bytes reserved for the EFI partition for GPT volumes
	efiPartitionSize = 209715200
)

var (
	// Dependency injection for testing.
	diskutilCmd = diskutil

	// Wrapped errors for testing.
	errDiskutil = errors.New(`diskutil error`)

	// Regex for device handling.
	regExPartition = regexp.MustCompile(`([a-zA-Z]+)\ds[0-9]`)
)

// plistDiskUtilList describes the results from diskutil list in plist format.
// Only the necessary fields are described. plist.Unmarshal ignores
// fields that aren't present.
type plistDiskUtilList struct {
	AllDisks              []string    `plist:"AllDisks"`
	AllDisksAndPartitions []plistDisk `plist:"AllDisksAndPartitions"`
}

// plistDisk describes the AllDisksAndPartitions field within the plist results
// from diskutil.
type plistDisk struct {
	Content          string           `plist:"Content"`
	DeviceIdentifier string           `plist:"DeviceIdentifier"`
	Partitions       []plistPartition `plist:"Partitions"`
	Size             int              `plist:"Size"`
}

// plistPartition describes the Partitions field within the plist results
// from diskutil.
type plistPartition struct {
	Content          string `plist:"Content"`
	DeviceIdentifier string `plist:"DeviceIdentifier"`
	MountPoint       string `plist:"MountPoint"`
	Size             uint64 `plist:"Size"`
	VolumeName       string `plist:"VolumeName"`
}

// plistDeviceInfo describes the information returned by the
// 'diskutil info' command.
type plistDeviceInfo struct {
	DeviceIdentifier string `plist:"DeviceIdentifier"`
	Path             string `plist:"DeviceNode"`
	Removable        bool   `plist:"RemovableMedia"`
	Size             uint64 `plist:"IOKitSize"`
	FullName         string `plist:"IORegistryEntryName"`
	ModelName        string `plist:"MediaName"`
	PartitionStyle   string `plist:"Content"`
}

// Search performs a device search based on the provided parameters and returns
// a slice of storage devices that meet the criteria. Parameters are not
// mandatory. For example, if no deviceID is passed, all deviceID's are
// considered for the search.
func Search(deviceID string, minSize, maxSize uint64, removableOnly bool) ([]*Device, error) {
	params := []string{"list", "-plist", "physical", deviceID}
	if deviceID == "" {
		params = []string{"list", "-plist", "physical"}
	}
	// Obtain disk information from diskutil.
	out, err := diskutilCmd(params...)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, errDiskutil)
	}
	pDevice := plistDiskUtilList{}
	err = plist.Unmarshal(out, &pDevice)
	if err != nil {
		return nil, fmt.Errorf("plist.Unmarshal returned %v: %w", err, errUnmarshal)
	}
	if len(pDevice.AllDisksAndPartitions) < 1 {
		return nil, fmt.Errorf("could not find any devices, got:%d, want:>0, %w", len(pDevice.AllDisksAndPartitions), errEmpty)
	}
	// Process the discovered devices. On Darwin, some needed information is
	// only provided by the diskutil info subcommand, so we shard out to that
	// command to obtain additional device information.
	found := []*Device{}
	for _, id := range pDevice.AllDisks {
		// Skip partitions. We're only interested in disk devices here.
		if regExPartition.Match([]byte(id)) {
			continue
		}
		device, err := New(id)
		if err != nil {
			return nil, fmt.Errorf("New(%q) returned %v: %w", id, err, errDetectDisk)
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
		found = append(found, device)
	}
	return found, nil
}

// New takes a unique device ID (e.g. disk1) and returns a pointer to a Device
// that describes the removable disk and its contents or an error.
func New(deviceID string) (*Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("empty device ID: %w", errInput)
	}
	// Obtain information about the disk, including manufacturer, make and model.
	out, err := diskutilCmd("info", "-plist", deviceID)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, errDiskutil)
	}
	pDevice := plistDeviceInfo{}
	if err = plist.Unmarshal(out, &pDevice); err != nil {
		return nil, fmt.Errorf("plist.Unmarshal returned %v: %w", err, errUnmarshal)
	}
	if pDevice == (plistDeviceInfo{}) {
		return nil, errDetectDisk
	}
	partStyle, ok := partStyles[pDevice.PartitionStyle]
	if !ok {
		partStyle = glstor.UnknownStyle
	}
	manufacturer := strings.Split(strings.Replace(pDevice.FullName, pDevice.ModelName, "", 1), " ")[0]

	// Build Device
	device := &Device{
		id:        pDevice.DeviceIdentifier,
		path:      pDevice.Path,
		removable: pDevice.Removable,
		size:      pDevice.Size,
		make:      manufacturer,
		model:     strings.TrimSpace(pDevice.ModelName),
		partStyle: partStyle,
	}
	// Add Partition Information
	if err := device.DetectPartitions(false); err != nil {
		return nil, fmt.Errorf("DetectPartitions() for %q returned %v: %w", device.Identifier(), err, errDetectDisk)
	}

	return device, nil
}

// DetectPartitions updates a device with known partition information on Darwin.
func (device *Device) DetectPartitions(mount bool) error {
	if device.id == "" {
		return fmt.Errorf("device ID was empty: %w", errInput)
	}
	// Pull partition information about the device from diskutil.
	out, err := diskutilCmd("list", "-plist", "physical", device.id)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errDiskutil)
	}
	// pDisk contains the results from diskutil.
	pDisk := plistDiskUtilList{}
	err = plist.Unmarshal(out, &pDisk)
	if err != nil {
		return fmt.Errorf("plist.Unmarshal returned %v: %w", err, errUnmarshal)
	}
	// No partitions might just mean we have an empty disk with no
	// filesystem, which can be expected and should not throw an error.
	if len(pDisk.AllDisksAndPartitions[0].Partitions) < 1 {
		return nil
	}

	// Process partitions and add them to the device.
	partitions := []Partition{}
	for _, part := range pDisk.AllDisksAndPartitions[0].Partitions {
		fs, ok := fileSystems[part.Content]
		if !ok {
			fs = UnknownFS
		}
		p := Partition{
			id:         part.DeviceIdentifier,
			path:       "/dev/" + part.DeviceIdentifier,
			mount:      part.MountPoint,
			label:      part.VolumeName,
			fileSystem: fs,
			size:       part.Size,
		}
		partitions = append(partitions, p)
	}
	device.partitions = partitions

	return nil
}

// Wipe removes all filesystem and partition table signatures from the
// device. The device is automatically converted to GPT style, as part
// of the wipe.
func (device *Device) Wipe() error {
	if device.id == "" {
		return errInput
	}
	// e.g.: diskutil eraseDisk FAT32 %noformat% GPT disk4
	// The noformat label signals to Darwin to skip the initialization (format) step.
	params := []string{"erasedisk", "FAT32", "%noformat%", "GPT", device.id}
	if out, err := diskutilCmd(params...); err != nil {
		return fmt.Errorf("diskutilCmd(%q) returned %q, %v: %w", params, out, err, errDiskutil)
	}
	// Update the receiver (device).
	device.partStyle = glstor.GptStyle
	device.partitions = []Partition{}

	return nil
}

// Partition repartitions and formats a device with a single FAT32 partition.
// On Darwin, the device is automatically formatted and mounted as part of the
// partitioning process, unless a blank label is passed.
func (device *Device) Partition(label string) error {
	if device.id == "" {
		return errInput
	}
	// A blank label is replaced by noformat. Darwin skips the formatting and
	// mounting for noformat.
	// Label should be uppercase otherwise diskutil will thow an error. (b/197085434)
	lblParam := strings.ToUpper(label)
	if label == "" {
		lblParam = "%noformat%"
	}
	// e.g.: diskutil partitionDisk disk5 1 GPT FAT32 EMPTY 100%
	params := []string{"partitionDisk", device.id, "1", "GPT", "FAT32", lblParam, "100%"}
	if out, err := diskutilCmd(params...); err != nil {
		return fmt.Errorf("diskutilCmd(%q) returned %q, %v: %w", params, out, err, errDiskutil)
	}

	// b/160276772 - Remove the EFI partition that is created, as some disk
	// utilities have trouble with it (e.g. manage-bde on Windows).
	// e.g.: diskutil eraseVolume free free disk2s1
	eParams := []string{"eraseVolume", "free", "free", device.id + "s1"}
	if out, err := diskutilCmd(eParams...); err != nil {
		return fmt.Errorf("diskutilCmd(%q) returned %q, %v: %w", eParams, out, err, errRemoval)
	}

	device.partStyle = glstor.GptStyle
	// Redetect the partitions to pickup the right mount names.
	if err := device.DetectPartitions(false); err != nil {
		return fmt.Errorf("DetectPartitions() for %q returned %v: %w", device.Identifier(), err, errPartition)
	}

	return nil
}

// Dismount unmounts all the volumes on the associated disk to limit accidental
// writes to these volumes. It is typically used with a flag when safety after
// writes to disk is desired.
func (device *Device) Dismount() error {
	if device.id == "" {
		return errInput
	}
	// e.g.: diskutil unmountDisk disk4
	if out, err := diskutilCmd("unmountDisk", "force", device.id); err != nil {
		return fmt.Errorf("%q, %v: %w", out, err, errDiskutil)
	}
	// Clear the mount locations from all partitions.
	for i := range device.partitions {
		device.partitions[i].mount = ""
	}
	return nil
}

// Eject takes a disk offline and makes it eligible for safe manual removal.
// This command is roughly equivalent to safely powering off a device. Because
// Ejecting a device also removes the disk from /dev, we take the additional
// step of clearing all the fields on the Device on completing this step to
// make it ineligible for further Device actions elsewhere in this library.
func (device *Device) Eject() error {
	if device.id == "" {
		return errInput
	}
	// e.g.: diskutil eject disk4
	if out, err := diskutilCmd("eject", device.id); err != nil {
		return fmt.Errorf("%q, %v: %w", out, err, errDiskutil)
	}
	// Wipe all information about the device as it is no longer available for
	// any device management actions.
	*device = Device{}

	return nil
}

// Mount makes a volume accessible under the /Volumes/[Label]. On Darwin,
// the base parameter is ignored. When multiple devices are attached to the
// system with the same label, Darwin automatically varies the label name.
// Thus, the mountpoint is not updated when Mount is called. Callers should
// run DetectPartition for the device to refresh the partition label.
func (part *Partition) Mount(base string) error {
	if part.id == "" {
		return fmt.Errorf("partition identifier was empty: %w", errInput)
	}
	if part.label == "" {
		return fmt.Errorf("partition label was empty: %w", errPartition)
	}
	// e.g.: diskutil mount disk4s2
	if out, err := diskutilCmd("mount", part.id); err != nil {
		return fmt.Errorf("diskutil mount %q returned %v %v: %w", part.id, out, err, errDiskutil)
	}

	return nil
}

// Format reformats a previously formatted partition with the same filesystem
// and label. On Darwin, Format fails if Partition was called on the device
// with an empty label, Callers who want to format a volume from scratch
// should call Partition instead.
func (part *Partition) Format(label string) error {
	if part.id == "" {
		return fmt.Errorf("partition identifier was empty: %w", errFormat)
	}
	// e.g.: diskutil reformat disk4s2
	if out, err := diskutilCmd("reformat", part.id); err != nil {
		return fmt.Errorf("diskutil reformat %q returned %v %v: %w", part.id, out, err, errDiskutil)
	}
	return nil
}

// diskutil represents the OS command used to manage available storage
// on a darwin or Mac OS system. An arbitrary set of arguments is accepted.
// The raw output is intended to be parsed by the caller, who should
// verify both the output and the error.
func diskutil(args ...string) ([]byte, error) {
	// e.g.: diskutil info -plist disk2
	out, err := exec.Command("diskutil", args...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("diskutil", %q) returned %q: %v`, args, out, err)
	}
	return out, nil
}
