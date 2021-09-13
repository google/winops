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
	"testing"

	glstor "github.com/google/glazier/go/storage"
)

type lsblkResult struct {
	BlockDevices []blockDevice `json:"blockdevices"`
}

var (
	sdb1 = blockDevice{
		Name:       "sdb1",
		Label:      "SOMELABEL",
		FSType:     "vfat",
		PTType:     "gpt",
		Type:       "part",
		Size:       30749491200,
		MountPoint: "/mnt/usb/128",
	}

	fakePartBD = &lsblkResult{
		[]blockDevice{sdb1},
	}
)

func (lr lsblkResult) json(t *testing.T) []byte {
	t.Helper()
	lrj, err := json.Marshal(lr)
	if err != nil {
		t.Errorf("json.Marshal() returned %v: ", err)
	}
	return lrj
}

func TestSearch(t *testing.T) {
	small := blockDevice{
		Name:    "sdb",
		PTType:  "gpt",
		Type:    "disk",
		Size:    eightGB - 1,
		HotPlug: true,
		Vendor:  "SanDisk",
		Model:   "Ultra_USB_3.0",
	}

	large := blockDevice{
		Name:    "sdc",
		PTType:  "gpt",
		Type:    "disk",
		Size:    eightGB + 1,
		HotPlug: true,
		Vendor:  "SanDisk",
		Model:   "Ultra_USB_3.0",
	}

	nonRemovable := blockDevice{
		Name:    "sde",
		PTType:  "gpt",
		Type:    "disk",
		Size:    eightGB,
		HotPlug: false,
		Vendor:  "Hitachi",
		Model:   "Deskstar",
	}

	largeDiskBD := &lsblkResult{
		[]blockDevice{large},
	}

	smallDiskBD := &lsblkResult{
		[]blockDevice{small},
	}

	nonRemovableBD := &lsblkResult{
		[]blockDevice{nonRemovable},
	}

	tests := []struct {
		desc             string
		fakeLsblkDiskCmd func(args ...string) ([]byte, error)
		fakeLsblkPartCmd func(args ...string) ([]byte, error)
		in               string // Represents a specific device to search for.
		min              uint64 // Represents the minSize parameter.
		max              uint64 // Represents the maxSize parameter.
		removable        bool   // Represents the removableOnly parameter.
		out              []*Device
		err              error
	}{
		{
			desc:             "error from lsblk",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return nil, errDetectDisk },
			in:               "sdb",
			err:              errDetectDisk,
		},
		{
			desc:             "unmarshall",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return badJSON, nil },
			in:               "sdb",
			err:              errUnmarshal,
		},
		{
			desc:             "removableOnly but none avaialable",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return nonRemovableBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdb",
			removable:        true,
			err:              nil,
		},
		{
			desc:             "no disks > minSize",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return smallDiskBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdb",
			min:              eightGB + 1,
			err:              nil,
		},
		{
			desc:             "no disks < maxSize",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return largeDiskBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdb",
			max:              eightGB - 1,
			err:              nil,
		},
		{
			desc:             "reading partitions",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return largeDiskBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return nil, errors.New("error") },
			in:               "sdb",
			err:              errPartRead,
		},
		{
			desc:             "success for single device",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return largeDiskBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdc",
			out: []*Device{
				&Device{
					id:        large.Name,
					path:      "/dev/" + large.Name,
					removable: large.HotPlug,
					size:      large.Size,
					make:      large.Vendor,
					model:     large.Model,
					partStyle: glstor.GptStyle,
					partitions: []Partition{
						{
							id:         "sdb1",
							path:       "/dev/sdb1",
							mount:      sdb1.MountPoint,
							label:      sdb1.Label,
							fileSystem: "FAT32",
							size:       sdb1.Size,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		lsblkDiskCmd = tt.fakeLsblkDiskCmd
		lsblkPartCmd = tt.fakeLsblkPartCmd
		got, err := Search(tt.in, tt.min, tt.max, tt.removable)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err: %v, want: %v", tt.desc, err, tt.err)
		}
		// It is generally sufficient to check that the device we expect is present
		// so we only compare the first result.
		if tt.out != nil {
			if err := cmpDevice(*got[0], *tt.out[0]); err != nil {
				t.Errorf("%s: %v", tt.desc, err)
			}
		}
	}
}

func TestNew(t *testing.T) {
	sdb := blockDevice{
		Name:    "sdb",
		PTType:  "gpt",
		Type:    "disk",
		Size:    123010547712,
		HotPlug: true,
		Vendor:  "SanDisk",
		Model:   "Ultra_USB_3.0",
	}

	sdc := blockDevice{
		Name:    "sdc",
		PTType:  "gpt",
		Type:    "disk",
		Size:    123010547712,
		HotPlug: true,
		Vendor:  "SanDisk",
		Model:   "Ultra_USB_3.0",
	}

	fakeDiskBD := &lsblkResult{[]blockDevice{sdb}}
	manyDisksBD := &lsblkResult{[]blockDevice{sdb, sdc}}

	tests := []struct {
		desc             string
		fakeLsblkDiskCmd func(args ...string) ([]byte, error)
		fakeLsblkPartCmd func(args ...string) ([]byte, error)
		in               string
		out              *Device
		err              error
	}{
		{
			desc: "empty deviceID",
			err:  errInput,
		},
		{
			desc:             "error from search",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return nil, errDetectDisk },
			in:               "sdb",
			err:              errDisk,
		},
		{
			desc:             "returned > 1 device",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return manyDisksBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdb",
			err:              errNoMatch,
		},
		{
			desc:             "success",
			fakeLsblkDiskCmd: func(args ...string) ([]byte, error) { return fakeDiskBD.json(t), nil },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			in:               "sdb",
			out: &Device{
				id:        sdb.Name,
				path:      "/dev/" + sdb.Name,
				removable: sdb.HotPlug,
				size:      sdb.Size,
				make:      sdb.Vendor,
				model:     sdb.Model,
				partStyle: glstor.GptStyle,
				partitions: []Partition{
					{
						id:         "",
						path:       "/dev/sdb1",
						mount:      "/mnt/usb/128",
						label:      "SOMELABEL",
						fileSystem: "FAT32",
						size:       30749491200,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		lsblkDiskCmd = tt.fakeLsblkDiskCmd
		lsblkPartCmd = tt.fakeLsblkPartCmd
		got, err := New(tt.in)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err: %v, want: %v", tt.desc, err, tt.err)
		}
		if tt.out != nil {
			if err := cmpDevice(*got, *tt.out); err != nil {
				t.Errorf("%s: %v", tt.desc, err)
			}
		}
	}
}

func TestDetectPartitions(t *testing.T) {
	tests := []struct {
		desc             string
		fakeLsblkPartCmd func(args ...string) ([]byte, error)
		device           *Device
		err              error
	}{
		{
			desc:   "empty deviceID",
			device: &Device{},
			err:    errInput,
		},
		{
			desc:             "lsblk error",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return nil, errLsblk },
			device:           &Device{id: "sdb"},
			err:              errLsblk,
		},
		{
			desc:             "unmarshal error",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return badJSON, nil },
			device:           &Device{id: "sdb"},
			err:              errUnmarshal,
		},
		{
			desc:             "success",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			device:           &Device{id: "sdb"},
		},
	}

	for _, tt := range tests {
		lsblkPartCmd = tt.fakeLsblkPartCmd
		err := tt.device.DetectPartitions(false)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err: %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestSelectPartition(t *testing.T) {
	goodBDsda := blockDevice{
		Name:       "sda1",
		Label:      "SOMELABEL",
		FSType:     "vfat",
		PTType:     "gpt",
		Type:       "part",
		Size:       eightGB + 1,
		MountPoint: "/mnt/usb/sda",
	}

	smallBD := blockDevice{
		Name:       "sdb1",
		Label:      "SOMELABEL",
		FSType:     "vfat",
		PTType:     "gpt",
		Type:       "part",
		Size:       eightGB - 1,
		MountPoint: "/mnt/usb/sdb",
	}

	ntfsBD := blockDevice{
		Name:       "sdc1",
		Label:      "SOMELABEL",
		FSType:     "Windows_NTFS",
		PTType:     "gpt",
		Type:       "part",
		Size:       eightGB + 1,
		MountPoint: "/mnt/usb/sdc",
	}

	emptyPartBD := &lsblkResult{
		[]blockDevice{},
	}

	smallPartBD := &lsblkResult{
		[]blockDevice{smallBD},
	}

	oneGoodPartBD := &lsblkResult{
		[]blockDevice{smallBD, goodBDsda},
	}

	noGoodPartBD := &lsblkResult{
		[]blockDevice{smallBD, ntfsBD},
	}

	tests := []struct {
		desc             string
		fakeLsblkPartCmd func(args ...string) ([]byte, error)
		device           *Device
		size             uint64
		label            string // Represents the label of the partition we expect.
		fs               FileSystem
		want             error
	}{
		{
			desc:             "detect partitions error",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return nil, fmt.Errorf("test") },
			device:           &Device{id: "sdc"},
			want:             errDisk,
		},
		{
			desc:             "no partitions",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return emptyPartBD.json(t), nil },
			device:           &Device{id: "sdc"},
			want:             errEmpty,
		},
		{
			desc:             "no partitions of proper size",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return smallPartBD.json(t), nil },
			device:           &Device{id: "sdc"},
			size:             eightGB,
			want:             errPartition,
		},
		{
			desc:             "fat32 partition too small",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return noGoodPartBD.json(t), nil },
			device:           &Device{id: "sdc"},
			size:             eightGB,
			fs:               FAT32,
			want:             errNoMatch,
		},
		{
			desc:             "one qualifying partition",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			device:           &Device{id: "sda"},
			size:             eightGB,
			label:            sdb1.MountPoint,
			want:             nil,
		},
		{
			desc:             "select second larger partition",
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return oneGoodPartBD.json(t), nil },
			device:           &Device{id: "sda"},
			size:             eightGB,
			label:            goodBDsda.MountPoint,
			want:             nil,
		},
	}

	for _, tt := range tests {
		lsblkPartCmd = tt.fakeLsblkPartCmd
		part, err := tt.device.SelectPartition(tt.size, tt.fs)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: SelectPartition(%d, %q) got = %q, want = %q", tt.desc, tt.size, tt.fs, err, tt.want)
		}
		if len(tt.device.partitions) >= 1 && part != nil {
			if part.mount != tt.label {
				t.Errorf("%s: unexpected default partition, got = %q, want = %q", tt.desc, part.mount, tt.label)
			}
		}
	}
}

func TestWipe(t *testing.T) {
	tests := []struct {
		desc             string
		fakeSudoCmd      func(args ...string) error
		fakeLsblkPartCmd func(args ...string) ([]byte, error) // DetectPartitions
		device           *Device
		err              error
	}{
		{
			desc:        "empty device path",
			fakeSudoCmd: nil,
			device:      &Device{},
			err:         errInput,
		},
		{
			desc:             "error from wipefs",
			fakeSudoCmd:      func(args ...string) error { return errors.New("error") },
			fakeLsblkPartCmd: func(args ...string) ([]byte, error) { return fakePartBD.json(t), nil },
			device:           &Device{id: "sdb", path: "/dev/sdb"},
			err:              errWipe,
		},
		{
			desc:        "success",
			fakeSudoCmd: func(args ...string) error { return nil },
			device:      &Device{path: "/dev/sdb"},
			err:         nil,
		},
	}

	for _, tt := range tests {
		sudoCmd = tt.fakeSudoCmd
		if err := tt.device.Wipe(); !errors.Is(err, tt.err) {
			t.Errorf("%s: device %v Wipe() = %v, want: %v", tt.desc, tt.device, err, tt.err)
		}
	}
}

func TestPartition(t *testing.T) {
	unpartitionedDevice := &Device{
		id:   "sdb",
		path: "/dev/sdb",
		size: eightGB,
	}

	goodPartition := Partition{
		id:         fmt.Sprintf(`%s1`, unpartitionedDevice.id),
		path:       fmt.Sprintf(`/dev/%s1`, unpartitionedDevice.id),
		fileSystem: FAT32,
		size:       unpartitionedDevice.size,
	}

	partitionedDevice := &Device{
		path:       "/dev/sdb",
		partitions: []Partition{goodPartition},
	}

	tests := []struct {
		desc          string
		fakeSudoCmd   func(args ...string) error
		device        *Device
		endPartStyle  glstor.PartitionStyle
		endPartitions []Partition
		err           error
	}{
		{
			desc:        "empty device path",
			fakeSudoCmd: nil,
			device:      &Device{},
			err:         errInput,
		},
		{
			desc:          "partitions exist at start",
			fakeSudoCmd:   nil,
			device:        partitionedDevice,
			endPartitions: []Partition{goodPartition},
			err:           errDisk,
		},
		{
			desc:        "error from parted",
			fakeSudoCmd: func(args ...string) error { return errors.New("error") },
			device:      &Device{path: "/dev/sdb"},
			err:         errSudo,
		},
		{
			desc:          "success",
			fakeSudoCmd:   func(args ...string) error { return nil },
			device:        unpartitionedDevice,
			endPartStyle:  glstor.GptStyle,
			endPartitions: []Partition{goodPartition},
			err:           nil,
		},
	}

	for _, tt := range tests {
		sudoCmd = tt.fakeSudoCmd
		err := tt.device.Partition("")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err = %v, want: %v", tt.desc, err, tt.err)
			continue
		}
		if tt.device.partStyle != tt.endPartStyle {
			t.Errorf("%s: got partSyle: %q, want: %q", tt.desc, tt.device.partStyle, tt.endPartStyle)
		}
		if err := cmpPartitions(tt.device.partitions, tt.endPartitions, 0); err != nil {
			t.Errorf("%s: cmpPartitions() mismatch = %v", tt.desc, err)
		}
	}
}

func TestDismount(t *testing.T) {
	tests := []struct {
		desc        string
		fakeSudoCmd func(args ...string) error
		device      *Device
		endMount    string
		err         error
	}{
		{
			desc:        "no partitions",
			fakeSudoCmd: nil,
			device:      &Device{path: "/dev/sdb"},
			err:         nil,
		},
		{
			desc:        "error from umount",
			fakeSudoCmd: func(args ...string) error { return errors.New("error") },
			device: &Device{
				path: "/dev/sdb",
				partitions: []Partition{
					Partition{mount: "/mnt"},
				},
			},
			endMount: "/mnt",
			err:      errSudo,
		},
		{
			desc:        "success",
			fakeSudoCmd: func(args ...string) error { return nil },
			device: &Device{
				path: "/dev/sdb",
				partitions: []Partition{
					Partition{
						path:  "/dev/sdb1",
						mount: "/mnt",
					},
				},
			},
			endMount: "",
			err:      nil,
		},
	}

	for _, tt := range tests {
		sudoCmd = tt.fakeSudoCmd
		err := tt.device.Dismount()
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err = %v, want: %v", tt.desc, err, tt.err)
		}
		for _, part := range tt.device.partitions {
			if part.mount != tt.endMount {
				t.Errorf("%s: partition %q, got: mounted at %q, want: mounted at %q", tt.desc, part.path, part.mount, tt.endMount)
			}
		}
	}
}

func TestMount(t *testing.T) {
	tests := []struct {
		desc        string
		fakeSudoCmd func(args ...string) error
		part        *Partition
		err         error
	}{
		{
			desc: "already mounted",
			part: &Partition{mount: "/test/mount"},
		},
		{
			desc: "unknown fs",
			part: &Partition{fileSystem: UnknownFS},
		},
		{
			desc: "empty path",
			part: &Partition{fileSystem: NTFS},
			err:  errInput,
		},
		{
			desc: "error creating mount directory",
			part: &Partition{id: "/s/db1", fileSystem: NTFS, path: "/test/path"},
			err:  errNotMounted,
		},
		{
			desc: "error mounting",
			fakeSudoCmd: func(args ...string) error {
				if args[0] == "mount" {
					return errors.New("error")
				}
				return nil
			},
			part: &Partition{fileSystem: NTFS, path: "/test/path"},
			err:  errSudo,
		},
		{
			desc:        "success",
			fakeSudoCmd: func(args ...string) error { return nil },
			part:        &Partition{fileSystem: NTFS, path: "/test/path"},
			err:         nil,
		},
	}

	for _, tt := range tests {
		sudoCmd = tt.fakeSudoCmd
		if err := tt.part.Mount(""); !errors.Is(err, tt.err) {
			t.Errorf("%s: Mount() = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		desc        string
		fakeSudoCmd func(args ...string) error
		parition    *Partition
		err         error
	}{
		{
			desc:        "partition path empty",
			fakeSudoCmd: nil,
			parition:    &Partition{path: ""},
			err:         errFormat,
		},
		{
			desc: "error from mkfs",
			fakeSudoCmd: func(args ...string) error {
				if args[0] == "mkfs" {
					return errors.New("error")
				}
				return nil
			},
			parition: &Partition{path: "/dev/sdb1"},
			err:      errSudo,
		},
		{
			desc: "error from fatlabel",
			fakeSudoCmd: func(args ...string) error {
				if args[0] == "fatlabel" {
					return errors.New("error")
				}
				return nil
			},
			parition: &Partition{path: "/dev/sdb1"},
			err:      errSudo,
		},
	}

	for _, tt := range tests {
		sudoCmd = tt.fakeSudoCmd
		err := tt.parition.Format("somelabel")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}
