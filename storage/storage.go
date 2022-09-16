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

// Package storage provides a unified method to access and modify to both fixed
// and removable disk storage. Implementations are kept consistent between
// platforms to permit testing through interfaces by consumers.
package storage

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	glstor "github.com/google/glazier/go/storage"
)

// FileSystem provides standardized file system descriptions (e.g. FAT32, NTFS)
type FileSystem string

const (
	// NTFS is the NTFS filesystem.
	NTFS FileSystem = "NTFS"
	// ExFAT is the ExFAT filesystem.
	ExFAT FileSystem = "exFAT"
	// FAT is the FAT filesystem.
	FAT FileSystem = "FAT"
	// FAT32 is the FAT32 filesystem.
	FAT32 FileSystem = "FAT32"
	// APFS is the Apple APFS filesystem.
	APFS FileSystem = "APFS"
	// UnknownFS is an unknown filesystem.
	UnknownFS FileSystem = "Unknown"

	// UnknownModel represents devices of unidentified models and makes.
	UnknownModel = "Unknown"
)

var (
	// partStyles maps the partitioning styles for platforms
	// to a standard set of values. Output is translated from:
	// linux   - lsblk (pttype column)
	// windows - powershell Get-Disk (PartitionStyle field)
	// darwin  - diskutil (Type field)
	partStyles = map[string]glstor.PartitionStyle{
		"dos":                    glstor.MbrStyle, // linux lsblk
		"gpt":                    glstor.GptStyle, // linux lsblk
		"GPT":                    glstor.GptStyle, // windows powershell
		"MBR":                    glstor.MbrStyle, // windows powershell
		"GUID_partition_scheme":  glstor.GptStyle, // darwin diskutil
		"FDisk_partition_scheme": glstor.MbrStyle, // darwin diskutil
	}

	// fileSystems maps the filesystem identifiers for platforms
	// to a standard set of values. output is translated from:
	// linux - lsblk (fstype column)
	// windows - powershell Get-Volume (FileSystem field)
	// darwin - diskutil (Type field)
	fileSystems = map[string]FileSystem{
		"vfat":                 FAT32, // linux lsblk
		"XINT13":               FAT,   // windows powershell
		"FAT32":                FAT32, // windows
		"FAT32 XINT13":         FAT32, // windows powershell
		"System":               FAT32, // windows powershell (EFI is typically also FAT32)
		"Basic":                FAT32, // windows powershell exFat or FAT32, both mountable
		"IFS":                  NTFS,  // windows powershell
		"Windows_NTFS":         NTFS,  // darwin diskutil (same for NTFS and exFAT)
		"Windows_FAT_32":       FAT32, // darwin diskutil
		"Microsoft Basic Data": FAT32, // darwin diskutil
		"EFI":                  FAT32, // darwin diskutil (linux formatted vFat on EFI)
		"Apple_APFS":           APFS,  // darwin diskutil
	}

	// Wrapped errors for testing.
	errDetectDisk   = errors.New("disk detection error")
	errDisk         = errors.New("device error")
	errEmpty        = errors.New("device empty")
	errFormat       = errors.New("formatting error")
	errInput        = errors.New("invalid or missing input")
	errNotEmpty     = errors.New("device not empty")
	errNotMounted   = errors.New("device not mounted")
	errNotRemovable = errors.New("no removable devices")
	errNoMatch      = errors.New("no match found")
	errPartition    = errors.New("partition error")
	errRead         = errors.New("read error")
	errRemoval      = errors.New("removal error")
	errUnmarshal    = errors.New("unmarshal error")
	errWipe         = errors.New("disk wipe error")
)

// Device describes a physical device that is currently
// attached to the system.
type Device struct {
	id        string // Unique identifier (e.g. sda or 0).
	path      string // Full path to the physical device.
	removable bool
	size      uint64
	make      string
	model     string

	// Partitioning Information
	// TODO (b/151981913) This remains a string for now to retain compatibility
	// with main. It will eventually be moved to type 'partition'.
	partStyle  glstor.PartitionStyle
	partitions []Partition
}

// Partition describes a disk partition using platform-independent paths and
// terminology. Partitions are considered immutable. If a partition is changed
// it should be redetected to ensure data integrity.
type Partition struct {
	disk       string // the disk identifier that this partition belongs to.
	id         string // Unique identifier (e.g. 1, sda1 or disk1s1)
	path       string
	mount      string
	label      string
	fileSystem FileSystem
	size       uint64
}

// Identifier returns a human-readable identifier for the device using the
// available device ID.
func (device *Device) Identifier() string {
	return device.id
}

// Size returns the size of the device in bytes.
func (device *Device) Size() uint64 {
	return device.size
}

// FriendlyName returns a human-readable friendly name for the device using
// available make and model information.
func (device *Device) FriendlyName() string {
	switch {
	case device.make == "" && device.model == "":
		return UnknownModel
	case device.make != "" && device.model == "":
		return device.make
	case device.make == "" && device.model != "":
		return device.model
	case device.make != "" && device.model != "":
		return device.make + " " + device.model
	}
	return UnknownModel
}

// Contents returns a list of the contents of a partition.
func (part *Partition) Contents() ([]string, error) {
	path := strings.TrimSpace(part.mount)
	if path == "" {
		return []string{}, errNotMounted
	}
	if runtime.GOOS == "windows" && !strings.Contains(path, `:\`) {
		path = path + `:\`
	}
	// Enumerate the contents.
	list, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadDir(%s) returned %v: %w", path, err, errInput)
	}
	var contents []string
	for _, f := range list {
		// Construct complete dir/dest paths
		fullPath := filepath.Join(part.mount, f.Name())
		contents = append(contents, fullPath)
	}
	return contents, nil
}

// SelectPartition enumerates the partitions on a device and returns the first
// partition that matches the criteria. Size and type are valid criteria. A size
// of zero is treated as "any size". A blank type is treated as "any type". If
// both input parameters are set to any, the first available partition is
// returned, or an error if there are no avaialble partitions.
func (device *Device) SelectPartition(minSize uint64, fs FileSystem) (*Partition, error) {
	// Refresh the partition table prior to scanning.
	assignMount := false
	if err := device.DetectPartitions(assignMount); err != nil {
		return nil, fmt.Errorf("device.detectPartitions() returned %v: %w", err, errDisk)
	}
	if len(device.partitions) < 1 {
		return nil, fmt.Errorf("no available partitions: %w", errEmpty)
	}
	available := []Partition{}
	for _, part := range device.partitions {
		if part.size >= minSize {
			available = append(available, part)
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no partitions > %d bytes were available: %w", minSize, errPartition)
	}
	// If no filesystem was specified, return the first avaialble partition.
	if fs == "" {
		return &available[0], nil
	}
	for _, part := range available {
		if part.fileSystem == fs {
			return &part, nil
		}
	}
	// The requested filesystem was not found among the avaialble partitions.
	return nil, fmt.Errorf("no available partitions of type %q were found: %w", fs, errNoMatch)
}

// Erase removes all files from a mounted partition. The erase operation is
// typically used when contents of a partition need to be refreshed, but a
// full reformat is not desirable, such as when a refresh is needed but the
// user is not running with elevated rights.
func (part *Partition) Erase() error {
	if part.mount == "" {
		return errNotMounted
	}
	path := part.mount
	if runtime.GOOS == "windows" && !strings.Contains(path, `:\`) {
		path = path + `:\`
	}
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("os.Open(%q) returned %v: %w", path, err, errDisk)
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("reading folders in %q returned %v: %w", path, err, errRead)
	}
	for _, folder := range names {
		p := filepath.Join(path, folder)
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("os.RemoveAll(%s) returned %v: %w", p, err, errWipe)
		}
	}
	return nil
}

// Identifier returns the identifier for the partition.
func (part *Partition) Identifier() string {
	return part.id
}

// Label returns the assigned label of the partition.
func (part *Partition) Label() string {
	return part.label
}

// MountPoint returns the mount point of the partition.
func (part *Partition) MountPoint() string {
	return part.mount
}
