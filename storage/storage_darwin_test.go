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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/groob/plist"
	glstor "github.com/google/glazier/go/storage"
)

var (
	// Shared disk and partition info
	emptyResult = plistDiskUtilList{
		AllDisksAndPartitions: []plistDisk{},
	}
	part1 = plistPartition{
		DeviceIdentifier: "disk1s1",
	}
	disk1 = plistDisk{
		DeviceIdentifier: "disk1",
		Partitions:       []plistPartition{part1},
	}
	disk1Info = plistDeviceInfo{
		DeviceIdentifier: "disk1",
		Size:             eightGB,
		Removable:        true,
	}
	goodDiskUtil = plistDiskUtilList{
		AllDisks:              []string{disk1.DeviceIdentifier},
		AllDisksAndPartitions: []plistDisk{disk1},
	}
)

func (d plistDeviceInfo) plist(t *testing.T) []byte {
	t.Helper()
	r, err := plist.Marshal(&d)
	if err != nil {
		t.Errorf("plist.Marshal() returned %v: ", err)
	}
	return r
}

func TestSearch(t *testing.T) {
	part2 := plistPartition{
		DeviceIdentifier: "disk2s1",
	}
	// disk2 is a non-removable disk
	disk2 := plistDisk{
		DeviceIdentifier: "disk2",
		Partitions:       []plistPartition{part2},
	}
	disk2Info := plistDeviceInfo{
		DeviceIdentifier: "disk2",
		Removable:        false,
	}
	fixedDiskUtil := plistDiskUtilList{
		AllDisks:              []string{disk2.DeviceIdentifier},
		AllDisksAndPartitions: []plistDisk{disk2},
	}

	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		in              string // Represents a specific device to search for.
		min             uint64 // Represents the minSize parameter.
		max             uint64 // Represents the maxSize parameter.
		removable       bool   // Represents the removableOnly parameter.
		out             []*Device
		err             error
	}{
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return nil, errors.New("error") },
			in:              "disk1",
			err:             errDiskutil,
		},
		{
			desc:            "unmarshall error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return badJSON, nil },
			in:              "disk1",
			err:             errUnmarshal,
		},
		{
			desc:            "no devices found",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return emptyResult.plist(t), nil },
			in:              "disk1",
			err:             errEmpty,
		},
		{
			desc: "error from New",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "info" {
						return []byte(""), errors.New("error")
					}
				}
				return goodDiskUtil.plist(t), nil
			},
			in:  "disk1",
			err: errDetectDisk,
		},
		{
			desc: "removableOnly but none avaialable",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "info" {
						return disk2Info.plist(t), nil
					}
				}
				return fixedDiskUtil.plist(t), nil
			},
			in:        "disk2",
			removable: true,
		},
		{
			desc: "no disks > minSize",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "info" {
						return disk1Info.plist(t), nil
					}
				}
				return goodDiskUtil.plist(t), nil
			},
			in:        "disk1",
			min:       eightGB + 1,
			removable: true,
		},
		{
			desc: "no disks < maxSize",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "info" {
						return disk1Info.plist(t), nil
					}
				}
				return goodDiskUtil.plist(t), nil
			},
			in:        "disk1",
			min:       eightGB - 1,
			removable: true,
		},
		{
			desc: "success",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "info" {
						return disk1Info.plist(t), nil
					}
				}
				return goodDiskUtil.plist(t), nil
			},
			in:        "disk1",
			removable: true,
			out: []*Device{
				&Device{
					id:        disk1Info.DeviceIdentifier,
					path:      disk1Info.Path,
					removable: disk1Info.Removable,
					size:      disk1Info.Size,
				},
			},
		},
	}
	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		got, err := Search(tt.in, tt.min, tt.max, tt.removable)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Search(%s, %d, %d, %t) err = %v, want: %v", tt.desc, tt.in, tt.min, tt.max, tt.removable, err, tt.err)
		}
		// It is generally sufficient to check that the device we expect is present
		// so we only compare the first result.
		if tt.out != nil {
			if err := cmpDevice(*got[0], *tt.out[0]); err != nil {
				t.Errorf("%s: cmpDevice() returned %v", tt.desc, err)
			}
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		in              string
		out             *Device
		err             error
	}{
		{
			desc: "empty deviceID",
			err:  errInput,
		},
		{
			desc:            "diskutil error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return nil, errors.New("error") },
			in:              "disk1",
			err:             errDiskutil,
		},
		{
			desc:            "unmarshall error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return badJSON, nil },
			in:              "disk1",
			err:             errUnmarshal,
		},
		{
			desc:            "disk detection error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return emptyResult.plist(t), nil },
			in:              "disk1",
			err:             errDetectDisk,
		},
		{
			desc: "detect partitions error",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "list" {
						return []byte(""), errors.New("error")
					}
				}
				return goodDiskUtil.plist(t), nil
			},
			in:  "disk1",
			err: errDetectDisk,
		},
		{
			desc: "success",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "list" {
						return goodDiskUtil.plist(t), nil
					}
				}
				return disk1Info.plist(t), nil
			},
			in: "disk1",
			out: &Device{
				id:        disk1Info.DeviceIdentifier,
				path:      disk1Info.Path,
				removable: disk1Info.Removable,
				size:      disk1Info.Size,
			},
		},
	}

	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		got, err := New(tt.in)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: New(%q) err = %v, want: %v", tt.desc, tt.in, err, tt.err)
		}
		if tt.out != nil {
			if err := cmpDevice(*got, *tt.out); err != nil {
				t.Errorf("%s: cmpDevice(%v, %v) returned %v", tt.desc, *got, *tt.out, err)
			}
		}
	}
}

func TestDetectPartitions(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		device          *Device
		err             error
	}{
		{
			desc:   "empty deviceID",
			device: &Device{},
			err:    errInput,
		},
		{
			desc:            "diskutil error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return nil, errors.New("error") },
			device:          &Device{id: "sdb"},
			err:             errDiskutil,
		},
		{
			desc:            "unmarshal error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return badJSON, nil },
			device:          &Device{id: "sdb"},
			err:             errUnmarshal,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return goodDiskUtil.plist(t), nil },
			device:          &Device{id: "sdb"},
		},
	}

	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.device.DetectPartitions(false)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err: %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func (l plistDiskUtilList) plist(t *testing.T) []byte {
	t.Helper()
	r, err := plist.Marshal(&l)
	if err != nil {
		t.Errorf("plist.Marshal() returned %v: ", err)
	}
	return r
}

func TestSelectPartition(t *testing.T) {
	goodPart := plistPartition{
		DeviceIdentifier: "disk1s1",
		Content:          "Windows_FAT_32",
		MountPoint:       "/Volumes/goodPart",
		Size:             eightGB,
		VolumeName:       "test",
	}

	goodDisk := plistDisk{
		DeviceIdentifier: "disk1",
		Partitions: []plistPartition{
			goodPart,
		},
	}

	smallPart := plistPartition{
		DeviceIdentifier: "disk2s1",
		Content:          "Windows_FAT_32",
		MountPoint:       "/Volumes/smallPart",
		Size:             eightGB - 1,
		VolumeName:       "test",
	}

	smallDisk := plistDisk{
		DeviceIdentifier: "disk2",
		Partitions: []plistPartition{
			smallPart,
		},
	}

	ntfsPart := plistPartition{
		DeviceIdentifier: "disk3s1",
		Content:          "Windows_NTFS",
		MountPoint:       "/Volumes/ntfsPart",
		Size:             eightGB,
		VolumeName:       "test",
	}

	smallFAT32 := plistDisk{
		DeviceIdentifier: "disk3",
		Partitions: []plistPartition{
			ntfsPart,
			smallPart,
		},
	}

	twoParts := plistDisk{
		DeviceIdentifier: "disk3",
		Partitions: []plistPartition{
			smallPart,
			goodPart,
		},
	}

	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		device          *Device
		size            uint64
		mount           string // Represents the drive letter of the partition we expect.
		fs              FileSystem
		want            error
	}{
		{
			desc:            "detect partitions error",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return nil, fmt.Errorf("test") },
			device:          &Device{},
			want:            errDisk,
		},
		{
			desc: "no partitions of proper size",
			fakeDiskutilCmd: func(...string) ([]byte, error) {
				return plistDiskUtilList{AllDisksAndPartitions: []plistDisk{smallDisk}}.plist(t), nil
			},
			device: &Device{id: "disk1"},
			size:   eightGB,
			want:   errPartition,
		},
		{
			desc: "fat32 partition too small",
			fakeDiskutilCmd: func(...string) ([]byte, error) {
				return plistDiskUtilList{AllDisksAndPartitions: []plistDisk{smallFAT32}}.plist(t), nil
			},
			device: &Device{id: "disk1"},
			size:   eightGB,
			fs:     FAT32,
			want:   errNoMatch,
		},
		{
			desc: "one qualifying partition",
			fakeDiskutilCmd: func(...string) ([]byte, error) {
				return plistDiskUtilList{AllDisksAndPartitions: []plistDisk{goodDisk}}.plist(t), nil
			},
			device: &Device{id: "disk1"},
			size:   eightGB,
			mount:  goodPart.MountPoint,
			want:   nil,
		},
		{
			desc: "select second larger partition",
			fakeDiskutilCmd: func(...string) ([]byte, error) {
				return plistDiskUtilList{AllDisksAndPartitions: []plistDisk{twoParts}}.plist(t), nil
			},
			device: &Device{id: "disk1"},
			size:   eightGB,
			mount:  goodPart.MountPoint,
			want:   nil,
		},
	}
	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		part, err := tt.device.SelectPartition(tt.size, tt.fs)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: SelectPartition(%d, %q) got = %q, want = %q", tt.desc, tt.size, tt.fs, err, tt.want)
		}
		if len(tt.device.partitions) >= 1 && part != nil {
			if part.mount != tt.mount {
				t.Errorf("%s: unexpected default partition, got = %q, want = %q", tt.desc, part.mount, tt.mount)
			}
		}
	}
}

func TestWipe(t *testing.T) {
	device := &Device{
		id:   "disk2",
		path: "/dev/disk2",
		size: eightGB,
	}
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		device          *Device
		wantPartStyle   glstor.PartitionStyle
		wantPartitions  []Partition
		err             error
	}{
		{
			desc:            "empty device id",
			fakeDiskutilCmd: nil,
			device:          &Device{},
			err:             errInput,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return []byte(""), errors.New("error") },
			device:          &Device{id: "disk2"},
			err:             errDiskutil,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return []byte(""), nil },
			device:          device,
			wantPartStyle:   glstor.GptStyle,
			wantPartitions:  []Partition{},
			err:             nil,
		},
	}

	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.device.Wipe()
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Wipe() err = %v, want: %v", tt.desc, err, tt.err)
			continue
		}
		if tt.device.partStyle != tt.wantPartStyle {
			t.Errorf("%s: Wipe() got partSyle: %q, want: %q", tt.desc, tt.device.partStyle, tt.wantPartStyle)
		}
		for i := range tt.wantPartitions {
			if err := cmpPartitions(tt.device.partitions, tt.wantPartitions, i); err != nil {
				t.Errorf("%s: Wipe() partition mismatch = %v", tt.desc, err)
			}
		}
	}
}

func TestPartition(t *testing.T) {
	part1 := plistPartition{
		DeviceIdentifier: "disk1s1",
	}
	disk1 := plistDisk{
		DeviceIdentifier: "disk1",
		Partitions:       []plistPartition{part1},
	}
	goodDiskUtil := plistDiskUtilList{
		AllDisksAndPartitions: []plistDisk{disk1},
	}

	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		label           string
		device          *Device
		err             error
	}{
		{
			desc:            "empty device id",
			fakeDiskutilCmd: nil,
			device:          &Device{},
			err:             errInput,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return []byte(""), errors.New("error") },
			device:          &Device{id: "disk2"},
			err:             errDiskutil,
		},
		{
			desc: "error from DetectPartitions eraseVolume",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "eraseVolume" {
						return []byte(""), errors.New("error")
					}
				}
				return []byte(""), nil
			},
			device: &Device{id: "disk2"},
			err:    errRemoval,
		},
		{
			desc: "error from DetectPartitions",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "list" {
						return []byte(""), errors.New("error")
					}
				}
				return []byte(""), nil
			},
			device: &Device{id: "disk2"},
			err:    errPartition,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return goodDiskUtil.plist(t), nil },
			device:          &Device{id: "disk2"},
			err:             nil,
		},
		{
			desc: "noformat is set",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "%noformat%" || arg == "list" || arg == "eraseVolume" {
						return goodDiskUtil.plist(t), nil
					}
				}
				return []byte(""), errors.New("noformat not present")
			},
			device: &Device{id: "disk2"},
			err:    nil,
		},
		{
			desc: "label is set",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) {
				for _, arg := range args {
					// Label will be translated to uppercase. (b/197085434)
					if arg == "SOMELABEL" || arg == "list" || arg == "eraseVolume" {
						return goodDiskUtil.plist(t), nil
					}
				}
				return []byte(""), errors.New("label not set correctly")
			},
			label:  "somelabel",
			device: &Device{id: "disk2"},
			err:    nil,
		},
	}

	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.device.Partition(tt.label)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Partition() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestDismount(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(...string) ([]byte, error)
		device          *Device
		wantMount       string
		err             error
	}{
		{
			desc:            "no device ID",
			fakeDiskutilCmd: nil,
			device:          &Device{},
			err:             errInput,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return []byte(""), errors.New("error") },
			device:          &Device{id: "disk2"},
			err:             errDiskutil,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(...string) ([]byte, error) { return []byte(""), nil },
			device: &Device{
				id: "disk2",
				partitions: []Partition{
					{
						id:    "disk2s1",
						mount: "/Volumes/Label1",
					},
					{
						id:    "disk2s2",
						mount: "/Volumes/Label2",
					},
				},
			},
			wantMount: "",
			err:       nil,
		},
	}

	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.device.Dismount()
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err = %v, want: %v", tt.desc, err, tt.err)
			continue
		}
		for _, part := range tt.device.partitions {
			if part.mount != tt.wantMount {
				t.Errorf("%s: partition %q, got: mounted at %q, want: mounted at %q", tt.desc, part.id, part.mount, tt.wantMount)
			}
		}
	}
}

func TestMount(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(args ...string) ([]byte, error)
		partition       *Partition
		err             error
	}{
		{
			desc:      "no part ID",
			partition: &Partition{},
			err:       errInput,
		},
		{
			desc:      "no label",
			partition: &Partition{id: "disk1"},
			err:       errPartition,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), errors.New("error") },
			partition:       &Partition{id: "disk1", label: "test"},
			err:             errDiskutil,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), nil },
			partition:       &Partition{id: "disk1", label: "test"},
			err:             nil,
		},
	}
	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.partition.Mount("")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Mount() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestEject(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(args ...string) ([]byte, error)
		device          *Device
		wantDevice      *Device
		err             error
	}{
		{
			desc:            "no device ID",
			fakeDiskutilCmd: nil,
			device:          &Device{},
			wantDevice:      &Device{},
			err:             errInput,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), errors.New("error") },
			device:          &Device{id: "disk2"},
			wantDevice:      &Device{id: "disk2"},
			err:             errDiskutil,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), nil },
			device:          &Device{id: "disk2"},
			wantDevice:      &Device{},
			err:             nil,
		},
	}
	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.device.Eject()
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Eject() err = %v, want: %v", tt.desc, err, tt.err)
		}
		if devCmp := cmp.Diff(tt.device, tt.wantDevice, cmp.AllowUnexported(Device{})); devCmp != "" {
			t.Errorf("%s: Eject() got: +, want: -\n%s", tt.desc, devCmp)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		desc            string
		fakeDiskutilCmd func(args ...string) ([]byte, error)
		partition       *Partition
		err             error
	}{
		{
			desc:      "no part ID",
			partition: &Partition{},
			err:       errFormat,
		},
		{
			desc:            "error from diskutil",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), errors.New("error") },
			partition:       &Partition{id: "disk1"},
			err:             errDiskutil,
		},
		{
			desc:            "success",
			fakeDiskutilCmd: func(args ...string) ([]byte, error) { return []byte(""), nil },
			partition:       &Partition{id: "disk1"},
			err:             nil,
		},
	}
	for _, tt := range tests {
		diskutilCmd = tt.fakeDiskutilCmd
		err := tt.partition.Format("")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Format() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}
