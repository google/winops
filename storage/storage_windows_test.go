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
	"strings"
	"testing"
)

var (
	disk1 = pGetPartitionResult{
		DiskNum:      1,
		DriveLetter:  "F",
		PartitionNum: 1,
		Path:         `\\?\Volume{78869425-9bc5-11ea-956e-f85971225104}\`,
		Size:         eightGB,
	}
)

func (r pGetPartitionResults) json(t *testing.T) []byte {
	t.Helper()
	j, err := json.Marshal(r)
	if err != nil {
		t.Errorf("json.Marshal() returned %v: ", err)
	}
	return j
}

func TestDetectPartitions(t *testing.T) {
	tests := []struct {
		desc              string
		fakePowershellCmd func(string) ([]byte, error)
		device            *Device
		err               error
	}{
		{
			desc:   "empty deviceID",
			device: &Device{},
			err:    errInput,
		},
		{
			desc: "powershell empty partition list error",
			fakePowershellCmd: func(string) ([]byte, error) {
				return []byte(`Get-Partition : No MSFT_Partition objects found with property`), nil
			},
			device: &Device{id: "disk1"},
			err:    nil,
		},
		{
			desc:              "powershell error",
			fakePowershellCmd: func(string) ([]byte, error) { return nil, errPowershell },
			device:            &Device{id: "disk1"},
			err:               errPowershell,
		},
		{
			desc:              "empty partition list",
			fakePowershellCmd: func(string) ([]byte, error) { return []byte{}, nil },
			device:            &Device{id: "disk1"},
			err:               errNoOutput,
		},
		{
			desc:              "unmarshal error",
			fakePowershellCmd: func(string) ([]byte, error) { return badJSON, nil },
			device:            &Device{id: "disk1"},
			err:               errUnmarshal,
		},
		{
			desc:              "success",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{disk1}.json(t), nil },
			device:            &Device{id: "disk1"},
		},
	}

	for _, tt := range tests {
		powershellCmd = tt.fakePowershellCmd
		err := tt.device.DetectPartitions(false)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: err: %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestSelectPartition(t *testing.T) {
	goodDisk := pGetPartitionResult{
		DiskNum:      1,
		DriveLetter:  "E",
		PartitionNum: 1,
		Path:         `\\?\Volume{78869425-9bc5-11ea-956e-f85971225104}\`,
		Size:         eightGB,
		Type:         "Basic",
	}

	smallDisk := pGetPartitionResult{
		DiskNum:      1,
		DriveLetter:  "F",
		PartitionNum: 1,
		Path:         `\\?\Volume{78869425-9bc5-11ea-956e-f85971225104}\`,
		Size:         eightGB - 1,
		Type:         "Basic",
	}

	ntfsDisk := pGetPartitionResult{
		DiskNum:      1,
		DriveLetter:  "G",
		PartitionNum: 1,
		Path:         `\\?\Volume{78869425-9bc5-11ea-956e-f85971225104}\`,
		Size:         eightGB,
		Type:         "Windows_NTFS",
	}

	tests := []struct {
		desc              string
		fakePowershellCmd func(string) ([]byte, error)
		device            *Device
		size              uint64
		mount             string // Represents the drive letter of the partition we expect.
		fs                FileSystem
		want              error
	}{
		{
			desc:              "detect partitions error",
			fakePowershellCmd: func(string) ([]byte, error) { return nil, fmt.Errorf("test") },
			device:            &Device{id: "1"},
			want:              errDisk,
		},
		{
			desc:              "no partitions",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{}.json(t), nil },
			device:            &Device{id: "1"},
			want:              errEmpty,
		},
		{
			desc:              "no partitions of proper size",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{smallDisk}.json(t), nil },
			device:            &Device{id: "1"},
			size:              eightGB,
			want:              errPartition,
		},
		{
			desc:              "fat32 partition too small",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{ntfsDisk, smallDisk}.json(t), nil },
			device:            &Device{id: "1"},
			size:              eightGB,
			fs:                FAT32,
			want:              errNoMatch,
		},
		{
			desc:              "one qualifying partition",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{goodDisk}.json(t), nil },
			device:            &Device{id: "1"},
			size:              eightGB,
			mount:             goodDisk.DriveLetter,
			want:              nil,
		},
		{
			desc:              "select second larger partition",
			fakePowershellCmd: func(string) ([]byte, error) { return pGetPartitionResults{smallDisk, goodDisk}.json(t), nil },
			device:            &Device{id: "sda"},
			size:              eightGB,
			mount:             goodDisk.DriveLetter,
			want:              nil,
		},
	}
	for _, tt := range tests {
		powershellCmd = tt.fakePowershellCmd
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
	tests := []struct {
		desc              string
		fakePowershellCmd func(string) ([]byte, error)
		device            *Device
		wantPartStyle     string
		wantPartitions    []Partition
		err               error
	}{
		{
			desc:   "empty device id",
			device: &Device{},
			err:    errInput,
		},
		{
			desc:              "error wiping",
			fakePowershellCmd: func(string) ([]byte, error) { return nil, errWipe },
			device:            &Device{id: "1"},
			err:               errWipe,
		},
		{
			desc:              "error from Clear-Disk",
			fakePowershellCmd: func(string) ([]byte, error) { return []byte("Clear-Disk error"), nil },
			device:            &Device{id: "1"},
			err:               errDisk,
		},
		{
			desc: "error set GPT",
			fakePowershellCmd: func(arg string) ([]byte, error) {
				if strings.Contains(arg, "Set-Disk") {
					return []byte(""), errors.New("error")
				}
				return nil, nil
			},
			device: &Device{id: "1"},
			err:    errPowershell,
		},
		{
			desc: "error from Set-Disk",
			fakePowershellCmd: func(arg string) ([]byte, error) {
				if strings.Contains(arg, "Set-Disk") {
					return []byte("Set-Disk error"), nil
				}
				return nil, nil
			},
			device: &Device{id: "1"},
			err:    errSetDisk,
		},
		{
			desc:              "success",
			fakePowershellCmd: func(string) ([]byte, error) { return nil, nil },
			device:            &Device{id: "1"},
			wantPartStyle:     string(gpt),
			wantPartitions:    []Partition{},
			err:               nil,
		},
	}

	for _, tt := range tests {
		powershellCmd = tt.fakePowershellCmd
		if err := tt.device.Wipe(); !errors.Is(err, tt.err) {
			t.Errorf("%s: Wipe for device.id %q = %v, want: %v", tt.desc, tt.device.id, err, tt.err)
			continue
		}
		if tt.device.partStyle != tt.wantPartStyle {
			t.Errorf("%s: Wipe for device.id %q got partSyle: %q, want: %q", tt.desc, tt.device.id, tt.device.partStyle, tt.wantPartStyle)
		}
		for i := range tt.wantPartitions {
			if err := cmpPartitions(tt.device.partitions, tt.wantPartitions, i); err != nil {
				t.Errorf("%s: Wipe for device.id %q partition mismatch = %v", tt.desc, tt.device.id, err)
			}
		}
	}
}

func TestFreeDrive(t *testing.T) {
	tests := []struct {
		desc                string
		fakeAvailableDrives func() []string
		in                  string
		out                 string
		err                 error
	}{
		{
			desc:                "no free drives",
			fakeAvailableDrives: func() []string { return []string{} },
			err:                 errOSResource,
		},
		{
			desc:                "success (requested available)",
			in:                  "Y",
			fakeAvailableDrives: func() []string { return []string{"X", "Y", "Z"} },
			out:                 "Y",
			err:                 nil,
		},
		{
			desc:                "requested unavailable",
			in:                  "F",
			fakeAvailableDrives: func() []string { return []string{"X", "Y", "Z"} },
			err:                 errDriveLetter,
		},
		{
			desc:                "success (none requested)",
			fakeAvailableDrives: func() []string { return []string{"X", "Y", "Z"} },
			out:                 "X",
			err:                 nil,
		},
	}
	for _, tt := range tests {
		availableDrives = tt.fakeAvailableDrives
		got, err := freeDrive(tt.in)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: freeDrive(%q) err = %v, want: %v", tt.desc, tt.in, err, tt.err)
			continue
		}
		if got != tt.out {
			t.Errorf("%s: freeDrive(%q) got = %q, want: %q", tt.desc, tt.in, got, tt.out)
		}
	}
}

func TestFormat(t *testing.T) {

	partition := &Partition{path: `\\?\Volume{78869425-9bc5-11ea-956e-f85971225104}\`}

	tests := []struct {
		desc                string
		fakePowershellCmd   func(string) ([]byte, error)
		fakeSetPartitionCmd func(string) ([]byte, error)
		fakeAvailableDrives func() []string
		partition           *Partition
		err                 error
	}{
		{
			desc:      "no part path",
			partition: &Partition{},
			err:       errInput,
		},
		{
			desc:              "error from powershell",
			fakePowershellCmd: func(string) ([]byte, error) { return []byte(""), errors.New("error") },
			partition:         partition,
			err:               errPowershell,
		},
		{
			desc:              "error from Format-Volume",
			fakePowershellCmd: func(string) ([]byte, error) { return []byte("Format-Volume error"), nil },
			partition:         partition,
			err:               errFormat,
		},
		{
			desc:                "error from Mount",
			fakePowershellCmd:   func(string) ([]byte, error) { return []byte(""), nil },
			fakeSetPartitionCmd: func(string) ([]byte, error) { return []byte(""), errors.New("error") },
			fakeAvailableDrives: func() []string { return nil },
			partition:           partition,
			err:                 errMount,
		},
		{
			desc:                "success",
			fakePowershellCmd:   func(string) ([]byte, error) { return []byte(""), nil },
			fakeSetPartitionCmd: func(string) ([]byte, error) { return []byte(""), nil },
			fakeAvailableDrives: func() []string { return []string{"F"} },
			partition:           partition,
			err:                 nil,
		},
	}
	for _, tt := range tests {
		powershellCmd = tt.fakePowershellCmd
		setPartitionCmd = tt.fakeSetPartitionCmd
		availableDrives = tt.fakeAvailableDrives
		err := tt.partition.Format("")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Format() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}
