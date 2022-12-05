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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	glstor "github.com/google/glazier/go/storage"
)

func TestDetectPartitions(t *testing.T) {
	errCon := errors.New("connect error")
	errGetParts := errors.New("GetPartitions error")
	errGetVols := errors.New("GetVolumes error")
	tests := []struct {
		desc        string
		device      *Device
		parts       []glstor.Partition
		errCon      error
		errGetParts error
		errGetVols  error
		wantErr     error
		wantParts   []Partition
	}{
		{
			desc:    "empty deviceID",
			device:  &Device{},
			wantErr: errInput,
		},
		{
			desc:    "connect error",
			device:  &Device{id: "disk1"},
			errCon:  errCon,
			wantErr: errCon,
		},
		{
			desc:        "get partition error",
			device:      &Device{id: "disk1"},
			errGetParts: errGetParts,
			wantErr:     errGetParts,
		},
		{
			desc:      "empty partition list",
			device:    &Device{id: "disk1"},
			parts:     []glstor.Partition{},
			wantParts: []Partition{},
		},
		{
			desc:   "get volumes error",
			device: &Device{id: "disk1"},
			parts: []glstor.Partition{glstor.Partition{
				DiskNumber:      1,
				PartitionNumber: 2,
				AccessPaths:     []string{`F:\`, `\\?\Volume{7b2d8681-0674-11ec-b6ba-401c8340851b}\`},
			}},
			errGetVols: errGetVols,
			wantErr:    errGetVols,
		},
		{
			desc:   "success",
			device: &Device{id: "disk1"},
			parts: []glstor.Partition{
				glstor.Partition{
					DiskNumber:      1,
					PartitionNumber: 1,
				},
				glstor.Partition{
					DiskNumber:      1,
					PartitionNumber: 2,
					AccessPaths:     []string{`F:\`, `\\?\Volume{7b2d8681-0674-11ec-b6ba-401c8340851b}\`},
				}},
			wantParts: []Partition{Partition{
				disk:       "1",
				id:         "2",
				path:       `\\?\Volume{7b2d8681-0674-11ec-b6ba-401c8340851b}\`,
				fileSystem: UnknownFS,
			}},
		},
	}
	for _, tt := range tests {
		fnConnect = func() (iService, error) {
			return &fakeService{}, tt.errCon
		}
		fnGetPartitions = func(path string, vp iService) (glstor.PartitionSet, error) {
			return glstor.PartitionSet{Partitions: tt.parts}, tt.errGetParts
		}
		fnGetVolumes = func(path string, vp iService) (glstor.VolumeSet, error) {
			return glstor.VolumeSet{Volumes: []glstor.Volume{glstor.Volume{}}}, tt.errGetVols
		}
		err := tt.device.DetectPartitions(false)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: err: got %v, want: %v", tt.desc, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.device.partitions, tt.wantParts, cmp.AllowUnexported(Partition{})); diff != "" {
			t.Errorf("%s: produced diff %v", tt.desc, diff)
		}
	}
}

/* TODO: SelectPartition is platform agnostic; move it to storage_test.go
 *
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
*/

func TestWipe(t *testing.T) {
	errCon := errors.New("Connect error")
	errGet := errors.New("GetVolumes error")
	errWipe := errors.New("Wipe error")
	tests := []struct {
		desc    string
		device  *Device
		errCon  error
		errGet  error
		errWipe error
		want    error
	}{
		{
			desc:   "no part path",
			device: &Device{},
			want:   errInput,
		},
		{
			desc:   "connection error",
			device: &Device{id: "1"},
			want:   errCon,
			errCon: errCon,
		},
		{
			desc:   "GetDisk error",
			device: &Device{id: "1"},
			want:   errGet,
			errGet: errGet,
		},
		{
			desc:   "Wipe error",
			device: &Device{id: "1"},
			want:   errWipe,
			errGet: errWipe,
		},
	}

	for _, tt := range tests {
		fnConnect = func() (iService, error) {
			return &fakeService{}, tt.errCon
		}
		fnGetDisks = func(query string, vp iService) (glstor.DiskSet, error) {
			return glstor.DiskSet{Disks: []glstor.Disk{glstor.Disk{}}}, tt.errGet
		}
		fnWipe = func(disk iDisk) error {
			return tt.errWipe
		}
		if err := tt.device.Wipe(); !errors.Is(err, tt.want) {
			t.Errorf("%s: Wipe() = %v, want: %v", tt.desc, err, tt.want)
			continue
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
	errCon := errors.New("Connect error")
	errGet := errors.New("GetVolumes error")
	errFmt := errors.New("Format error")
	tests := []struct {
		desc      string
		partition *Partition
		errCon    error
		errGet    error
		errFmt    error
		err       error
	}{
		{
			desc:      "no part path",
			partition: &Partition{},
			err:       errInput,
		},
		{
			desc:      "connection error",
			partition: &Partition{path: "\\.\foo\bar"},
			err:       errCon,
			errCon:    errCon,
		},
		{
			desc:      "GetVolumes error",
			partition: &Partition{path: "\\.\foo\bar"},
			err:       errGet,
			errGet:    errGet,
		},
		{
			desc:      "Format error",
			partition: &Partition{path: "\\.\foo\bar"},
			err:       errFmt,
			errFmt:    errFmt,
		},
	}
	for _, tt := range tests {
		fnConnect = func() (iService, error) {
			return &fakeService{}, tt.errCon
		}
		fnGetVolumes = func(path string, vp iService) (glstor.VolumeSet, error) {
			return glstor.VolumeSet{Volumes: []glstor.Volume{glstor.Volume{}}}, tt.errGet
		}
		fnFormat = func(fs FileSystem, label string, vol iVolume) error {
			return tt.errFmt
		}
		err := tt.partition.Format("")
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Format() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

type fakeService struct {
	ds    glstor.DiskSet
	ps    glstor.PartitionSet
	vs    glstor.VolumeSet
	input string
	err   error
}

func (fg *fakeService) Close() {}

func (fg *fakeService) GetDisks(in string) (glstor.DiskSet, error) {
	fg.input = in
	return fg.ds, fg.err
}

func (fg *fakeService) GetPartitions(in string) (glstor.PartitionSet, error) {
	fg.input = in
	return fg.ps, fg.err
}

func (fg *fakeService) GetVolumes(in string) (glstor.VolumeSet, error) {
	fg.input = in
	return fg.vs, fg.err
}

func TestWindowsGetVolumes(t *testing.T) {
	vs1 := glstor.VolumeSet{Volumes: []glstor.Volume{glstor.Volume{}}}
	err1 := errors.New("error1")
	tests := []struct {
		desc     string
		path     string
		inErr    error
		vols     glstor.VolumeSet
		wantVols glstor.VolumeSet
		wantErr  error
	}{
		{
			desc:     "empty",
			path:     "",
			vols:     glstor.VolumeSet{},
			inErr:    nil,
			wantVols: glstor.VolumeSet{},
			wantErr:  errDetectVol,
		},
		{
			desc:     "one volume",
			path:     "",
			vols:     vs1,
			inErr:    nil,
			wantVols: vs1,
			wantErr:  nil,
		},
		{
			desc:     "GetVolumes error",
			path:     "",
			vols:     glstor.VolumeSet{},
			inErr:    err1,
			wantVols: glstor.VolumeSet{},
			wantErr:  err1,
		},
	}
	for _, tt := range tests {
		got, err := windowsGetVolumes(tt.path, &fakeService{vs: tt.vols, err: tt.inErr})
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: windowsGetVolumes() err = %v, want: %v", tt.desc, err, tt.wantErr)
		}
		if !cmp.Equal(got, tt.wantVols, cmpopts.IgnoreUnexported(glstor.Volume{})) {
			t.Errorf("%s: windowsGetVolumes() got: %v, want: %v", tt.desc, got, tt.wantVols)
		}
	}
}

func TestWindowsGetVolumesEscaping(t *testing.T) {
	want := `WHERE Path='\\\\?\\Volume{a11dd3fc-b4d2-11e9-b27d-3cf01167ff7e}\\'`
	fs := &fakeService{}
	windowsGetVolumes(`\\?\Volume{a11dd3fc-b4d2-11e9-b27d-3cf01167ff7e}\`, fs)
	if diff := cmp.Diff(fs.input, want); diff != "" {
		t.Errorf("windowsGetVolumes() produced unexpected diff: %v", diff)
	}
}

var (
	errNTFS  = errors.New("ntfs error")
	errFAT32 = errors.New("fat32 error")
)

type FakeVol struct {
	err error
}

func (f *FakeVol) FormatFAT32(label string, allocationUnitSize int32, full, force bool) (glstor.Volume, glstor.ExtendedStatus, error) {
	var err error
	if f.err != nil {
		err = errFAT32
	}
	return glstor.Volume{}, glstor.ExtendedStatus{}, err
}
func (f *FakeVol) FormatNTFS(label string, allocationUnitSize int32, full, force, compress, shortFileNameSupport, useLargeFRS, disableHeatGathering bool) (glstor.Volume, glstor.ExtendedStatus, error) {
	var err error
	if f.err != nil {
		err = errNTFS
	}
	return glstor.Volume{}, glstor.ExtendedStatus{}, err
}

func TestWindowsFormat(t *testing.T) {
	tests := []struct {
		desc  string
		fs    FileSystem
		inErr error
		want  error
	}{
		{
			desc:  "fat32 ok",
			fs:    FAT32,
			inErr: nil,
			want:  nil,
		},
		{
			desc:  "fat32 err",
			fs:    FAT32,
			inErr: errors.New("formatting error"),
			want:  errFAT32,
		},
		{
			desc:  "ntfs ok",
			fs:    NTFS,
			inErr: nil,
			want:  nil,
		},
		{
			desc:  "ntfs err",
			fs:    NTFS,
			inErr: errors.New("formatting error"),
			want:  errNTFS,
		},
		{
			desc:  "unsupported fs",
			fs:    UnknownFS,
			inErr: nil,
			want:  errUnsupportedFileSystem,
		},
	}
	for _, tt := range tests {
		err := windowsFormat(tt.fs, tt.desc, &FakeVol{err: tt.inErr})
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: windowsFormat() err = %v, want: %v", tt.desc, err, tt.want)
		}
	}
}

func TestWindowsInitialize(t *testing.T) {
	errConv := errors.New("ConvertStyle error")
	tests := []struct {
		desc    string
		errConv error
		want    error
	}{
		{
			desc:    "ConvertStyle error",
			errConv: errConv,
			want:    errConv,
		},
		{
			desc: "success",
			want: nil,
		},
	}
	for _, tt := range tests {
		err := windowsInitialize(&FakeDisk{errConv: tt.errConv})
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: windowsInitialize() err = %v, want: %v", tt.desc, err, tt.want)
		}
	}
}

func TestWindowsPartition(t *testing.T) {
	errPart := errors.New("Partition error")
	tests := []struct {
		desc    string
		errPart error
		want    error
	}{
		{
			desc:    "Partition error",
			errPart: errPart,
			want:    errPart,
		},
		{
			desc: "success",
			want: nil,
		},
	}
	for _, tt := range tests {
		err := windowsPartition(&FakeDisk{errPart: tt.errPart}, 0, false, glstor.GptTypes.BasicData)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: windowsPartition() err = %v, want: %v", tt.desc, err, tt.want)
		}
	}
}

func TestWindowsWipe(t *testing.T) {
	errClear := errors.New("Clear error")
	tests := []struct {
		desc     string
		errClear error
		want     error
	}{
		{
			desc:     "Clear error",
			errClear: errClear,
			want:     errClear,
		},
		{
			desc: "success",
			want: nil,
		},
	}
	for _, tt := range tests {
		err := windowsWipe(&FakeDisk{errClear: tt.errClear})
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: windowsWipe() err = %v, want: %v", tt.desc, err, tt.want)
		}
	}
}

type FakeDisk struct {
	glstor.Disk

	errClear      error
	errConv       error
	errInitialize error
	errPart       error
}

func (f *FakeDisk) Close() {}

func (f *FakeDisk) Clear(removeData, removeOEM, zeroDisk bool) (glstor.ExtendedStatus, error) {
	return glstor.ExtendedStatus{}, f.errClear
}

func (f *FakeDisk) CreatePartition(size uint64, useMaximumSize bool, offset uint64, alignment int, driveLetter string, assignDriveLetter bool,
	mbrType *glstor.MbrType, gptType *glstor.GptType, hidden, active bool) (glstor.Partition, glstor.ExtendedStatus, error) {
	return glstor.Partition{}, glstor.ExtendedStatus{}, f.errPart
}

func (f *FakeDisk) Initialize(ps glstor.PartitionStyle) (glstor.ExtendedStatus, error) {
	return glstor.ExtendedStatus{}, f.errInitialize
}

func (f *FakeDisk) ConvertStyle(style glstor.PartitionStyle) (glstor.ExtendedStatus, error) {
	return glstor.ExtendedStatus{}, f.errConv
}
