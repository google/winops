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

//go:build linux
// +build linux

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus"
	glstor "github.com/google/glazier/go/storage"
)

var (
	// Wrapped errors for testing.
	errLsblk    = errors.New(`lsblk error`)
	errPartRead = errors.New(`partition reading error`)
	errFile     = errors.New(`file error`)
	errSudo     = errors.New(`sudo error`)

	// Dependency injection for testing.
	lsblkDiskCmd = lsblk
	lsblkPartCmd = lsblk
	sudoCmd      = sudo
)

// blockDevice models an individual result from lsblk.
type blockDevice struct {
	Name       string `json:"kname"`
	Label      string `json:"label"`
	FSType     string `json:"fstype"`
	PTType     string `json:"pttype"`
	Type       string `json:"type"`
	Size       uint64 `json:"size"`
	HotPlug    bool   `json:"hotplug"`
	MountPoint string `json:"mountpoint"`
	Vendor     string `json:"vendor"`
	Model      string `json:"model"`
}

// lsblk represents the OS command used to list block devices
// on a linux system. An arbitrary set of arguments is accepted.
// The raw output is intended to be parsed by the caller, who should
// verify both the output and the error.
func lsblk(args ...string) ([]byte, error) {
	out, err := exec.Command("lsblk", args...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("lsblk", %q) returned %q: %v`, args, out, err)
	}
	return out, nil
}

// Search performs a device search based on the provided parameters and returns
// a slice of storage devices that meet the criteria. Parameters are not
// mandatory. For example, if no deviceID is passed, all deviceID's are
// considered for the search.
func Search(deviceID string, minSize, maxSize uint64, removableOnly bool) ([]*Device, error) {
	params := []string{"-d", "-o", "kname,label,fstype,pttype,type,size,hotplug,mountpoint,vendor,model", "-b", "-J", "/dev/" + deviceID}
	if deviceID == "" {
		params = []string{"-d", "-o", "kname,label,fstype,pttype,type,size,hotplug,mountpoint,vendor,model", "-b", "-J"}
	}
	// Obtain details about the device from lsblk
	out, err := lsblkDiskCmd(params...)
	if err != nil {
		return nil, fmt.Errorf("lsblk returned %v: %w", err, errDetectDisk)
	}
	result := &struct {
		BlockDevices []blockDevice `json:"blockdevices"`
	}{}
	if err := json.Unmarshal(out, result); err != nil {
		return nil, fmt.Errorf("json.Unmarshal(%s) returned %v: %w", out, err, errUnmarshal)
	}
	found := []*Device{}
	for _, bd := range result.BlockDevices {
		partStyle, ok := partStyles[bd.PTType]
		if !ok {
			partStyle = glstor.UnknownStyle
		}
		// Build Device
		device := &Device{
			id:        bd.Name,
			path:      "/dev/" + bd.Name,
			removable: bd.HotPlug,
			size:      bd.Size,
			make:      strings.TrimSpace(bd.Vendor),
			model:     strings.TrimSpace(bd.Model),
			partStyle: partStyle,
		}
		// Add Partition Information
		if err := device.DetectPartitions(false); err != nil {
			return nil, fmt.Errorf("DetectPartitions(false) returned %v: %w", err, errPartRead)
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

// New takes a unique device ID (e.g. sda) and returns a pointer to a Device
// that describes the removable disk and its contents or an error.
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

// DetectPartitions updates a device with known partition information on Linux.
func (device *Device) DetectPartitions(mount bool) error {
	if device.id == "" {
		return fmt.Errorf("device ID was empty: %w", errInput)
	}

	// Pull block information for the device.
	out, err := lsblkPartCmd("-o", "kname,label,fstype,pttype,type,size,hotplug,mountpoint,vendor,model", "-b", "-J", "/dev/"+device.id)
	if err != nil {
		return fmt.Errorf("lablk returned %v: %w", err, errLsblk)
	}
	// result models the JSON formatted results from lsblk.
	result := &struct {
		BlockDevices []blockDevice `json:"blockdevices"`
	}{}
	if err := json.Unmarshal(out, result); err != nil {
		return fmt.Errorf("json.Unmarshal() returned %v: %w", err, errUnmarshal)
	}

	// Filter to just partitions.
	var partList []blockDevice
	for _, bd := range result.BlockDevices {
		if bd.Type == "part" {
			partList = append(partList, bd)
		}
	}
	// No partitions might just mean we have an empty disk with no
	// filesystem, which can be expected and should not throw an error.
	if len(partList) < 1 {
		return nil
	}

	// Process the partitions and add them to the device.
	partitions := []Partition{}
	for _, part := range partList {
		fs, ok := fileSystems[part.FSType]
		if !ok {
			fs = UnknownFS
		}
		p := Partition{
			id:         part.Name,
			path:       "/dev/" + part.Name,
			mount:      part.MountPoint,
			label:      part.Label,
			fileSystem: fs,
			size:       uint64(part.Size),
		}
		partitions = append(partitions, p)
	}
	device.partitions = partitions
	return nil
}

// Wipe removes all filesystem and partition table signatures from the device.
func (device *Device) Wipe() error {
	if device.path == "" {
		return fmt.Errorf("device path was empty: %w", errInput)
	}
	// If the device has mounted partitions, unmount them first.
	if err := device.Dismount(); err != nil {
		return fmt.Errorf("Unmount() for %q returned %v, %w", device.Identifier(), err, errDisk)
	}
	// Perform the wipe
	args := []string{"wipefs", "-a", device.path}
	if err := sudoCmd(args...); err != nil {
		return fmt.Errorf("sudoCmd(%v) returned %v: %w", args, err, errWipe)
	}

	// Update the receiver (device).
	device.partStyle = glstor.UnknownStyle
	device.partitions = []Partition{}

	return nil
}

// Partition sets up a GPT-Style partition scheme, and creates a single FAT32
// partition using the maximum amount of available space. Linux automatically
// assigns a GPT GUID (System or EFI) to the partition, meaning it does not
// automatically mount on Windows, however this is handled automatically in
// this library for Windows.
func (device *Device) Partition(label string) error {
	if device.path == "" {
		return errInput
	}
	if len(device.partitions) != 0 {
		return fmt.Errorf("partition table not empty: %w", errDisk)
	}
	// e.g.: sudo parted --script /dev/sdc -- mklabel gpt \ mkpart LBLNAME fat32 4096s 100% \ set 1 msftdata on \ p
	args := []string{"parted", "--script", device.path, "--", "mklabel", "gpt", "mkpart", label, "fat32", "4096s", "100%", "set", "1", "msftdata", "on", "p"}
	if err := sudoCmd(args...); err != nil {
		return fmt.Errorf("%v: %w", err, errSudo)
	}
	// Update the disk with the new partition information and partition style.
	device.partStyle = glstor.GptStyle
	device.partitions = []Partition{
		Partition{
			id:         fmt.Sprintf(`%s1`, device.id),
			path:       fmt.Sprintf(`/dev/%s1`, device.id),
			fileSystem: FAT32,
			size:       device.size,
		},
	}
	return nil
}

// Dismount enumerates all the mounted partitions on the device and unmounts
// them. It is typically used with a flag when safety after writes to disk are
// desired. Errors are only thrown when a mountpoint is found but cannot be
// unmounted.
func (device *Device) Dismount() error {
	if len(device.partitions) < 1 {
		return nil
	}
	for i, part := range device.partitions {
		// Skip partitions with no corresponding mountpoint.
		if part.mount == "" {
			continue
		}
		// e.g.: sudo umount /mnt/media
		if err := sudoCmd("umount", part.mount); err != nil {
			return fmt.Errorf("sudo umount %q returned %v: %w", part.mount, err, errSudo)
		}
		// Cleanup the mountpoint folder if this library created it.
		if strings.Contains(part.mount, part.id) {
			if err := os.RemoveAll(part.mount); err != nil {
				return fmt.Errorf("os.RemoveAll(%s) returned %v: %w", part.mount, err, errFile)
			}
		}
		// Update the device partition data.
		device.partitions[i].mount = ""
	}
	return nil
}

// Eject powers off the device.
func (device *Device) Eject() error {
	if device.path == "" {
		return errInput
	}
	// Open a connection to dbus
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to system D-Bus: %w", err)
	}

	// Get the org.freedesktop.UDisks2.Block.Drive property:
	// http://storaged.org/doc/udisks2-api/latest/gdbus-org.freedesktop.UDisks2.Block.html#gdbus-property-org-freedesktop-UDisks2-Block.Drive
	dObj := conn.Object("org.freedesktop.UDisks2", dbus.ObjectPath(device.path))
	ret, err := dObj.GetProperty("org.freedesktop.UDisks2.Block.Drive")
	if err != nil {
		return fmt.Errorf("failed to get device object path for block device object path %q: %w", device.path, err)
	}
	var drive dbus.ObjectPath
	if err := dbus.Store([]interface{}{ret}, &drive); err != nil {
		return fmt.Errorf("unexpected return value for g.freedesktop.UDisks2.Block.Drive: %v", err)
	}

	// Do org.freedesktop.UDisks2.Drive.PowerOff method call:
	// http://storaged.org/doc/udisks2-api/latest/gdbus-org.freedesktop.UDisks2.Drive.html#gdbus-method-org-freedesktop-UDisks2-Drive.PowerOff
	obj := conn.Object("org.freedesktop.UDisks2", dbus.ObjectPath(drive))
	options := map[string]dbus.Variant{"auth.no_user_interaction": dbus.MakeVariant(true)}
	call := obj.Call("org.freedesktop.UDisks2.Drive.PowerOff", 0, options)
	if call.Err != nil {
		return fmt.Errorf("failed to power down device with object path %q: %w", drive, call.Err)
	}
	return nil
}

// Mount reads the current mount point for the partition. If it is empty and
// the file system is readable, the device is mounted. If the device is already
// mounted, we simply do nothing and return.
func (part *Partition) Mount(base string) error {
	// Skip partitions that are already mounted.
	if part.mount != "" {
		return nil
	}
	// Skip partitions that cannot be mounted.
	// TODO(b/130833261) Revisit not throwing an error here during OSS.
	if part.fileSystem == UnknownFS {
		return nil
	}
	// Check that the path is present.
	if part.path == "" {
		return fmt.Errorf("partition path is empty: %w", errInput)
	}
	// The base path for the mount point can be optionally specified by the user.
	// We always use a randomly generated mount folder to avoid conflicts.
	mntPath, err := ioutil.TempDir(base, part.id)
	if err != nil {
		return fmt.Errorf("ioutil.TempDir('', %s) returned %v: %w", part.id, err, errNotMounted)
	}
	// Perform the mount and update the partition with the mount path.
	args := []string{"mount", "--options", "rw,users,umask=000", part.path, mntPath}
	if err := sudoCmd(args...); err != nil {
		return fmt.Errorf("sudoCmd(%v) returned %v: %w", args, err, errSudo)
	}
	part.mount = mntPath

	return nil
}

// Format formats the corresponding partition as vfat/FAT32 and sets the
// partition label.
func (part *Partition) Format(label string) error {
	if part.path == "" {
		return fmt.Errorf("partition path was empty: %w", errFormat)
	}
	// e.g.: sudo mkfs -t vfat /dev/sdc1
	mkfsArgs := []string{"mkfs", "-t", "vfat", part.path}
	if err := sudoCmd(mkfsArgs...); err != nil {
		return fmt.Errorf("sudoCmd(%v) returned %v: %w", mkfsArgs, err, errSudo)
	}
	// e.g.: sudo fatlabel /dev/sdc1 SOMELABEL
	lblArgs := []string{"fatlabel", part.path, label}
	if err := sudoCmd(lblArgs...); err != nil {
		return fmt.Errorf("sudoCmd(%v) returned %v: %w", lblArgs, err, errSudo)
	}
	return nil
}

// sudo represents the OS command used to run a command with elevated
// permissions on a linux system. Output is only used for error handling.
// If the command returns an error, it is returned with the output.
func sudo(args ...string) error {
	out, err := exec.Command("sudo", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf(`exec.Command(sudo %q) returned %q: %v`, args, out, err)
	}
	return nil
}
