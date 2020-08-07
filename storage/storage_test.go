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

package storage

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

const eightGB = 8589934592

var badJSON = []byte("bad JSON")

// cmpDevice is a partial custom comparer for select unexported fields of
// Device. Errors describing fields that do not match are returned. When all
// checked fields are equal, nil is returned for the checked fields.
func cmpDevice(got, want Device) error {
	if got.id != want.id {
		return fmt.Errorf("device id mismatch, got: %q, want: %q", got.id, want.id)
	}
	if got.path != want.path {
		return fmt.Errorf("device path mismatch, got: %q, want: %q", got.path, want.path)
	}
	if got.removable != want.removable {
		return fmt.Errorf("device removable mismatch, got: %t, want: %t", got.removable, want.removable)
	}
	if got.size != want.size {
		return fmt.Errorf("device size mismatch, got: %d, want: %d", got.size, want.size)
	}
	if got.make != want.make {
		return fmt.Errorf("device make mismatch, got: %q, want: %q", got.make, want.make)
	}
	if got.model != want.model {
		return fmt.Errorf("device model mismatch, got: %q, want: %q", got.model, want.model)
	}
	return nil
}

func TestIdentifier(t *testing.T) {
	want := "testID"
	device := Device{id: want}
	if got := device.Identifier(); got != want {
		t.Errorf("Identifier() got = %q, want = %q", got, want)
	}
}

func TestSize(t *testing.T) {
	want := uint64(123456789987654321)
	device := Device{size: want}
	if got := device.Size(); got != want {
		t.Errorf("Size() got = %q, want = %q", got, want)
	}
}

func TestFriendlyName(t *testing.T) {
	tests := []struct {
		desc  string
		make  string
		model string
		want  string
	}{
		{
			desc: "empty make and model",
			want: UnknownModel,
		},
		{
			desc: "make only",
			make: "foo maker",
			want: "foo maker",
		},
		{
			desc:  "model only",
			model: "super duper drive",
			want:  "super duper drive",
		},
		{
			desc:  "make and model",
			make:  "foo maker",
			model: "super duper drive",
			want:  "foo maker super duper drive",
		},
	}
	for _, tt := range tests {
		d := Device{make: tt.make, model: tt.model}
		if got := d.FriendlyName(); got != tt.want {
			t.Errorf("%s: FriendlyName() got = %q, want = %q", tt.desc, got, tt.want)
		}
	}
}

// fakeFileSystems returns empty and a non-empty temporary
// folders for testing purposes to simulate mounted filesystems.
// The caller is responsible for cleaning up the folders after
// their tests are complete.
func fakeFileSystems() (string, string, error) {
	empty, err := ioutil.TempDir("", "")
	if err != nil {
		return "", "", fmt.Errorf("ioutil.TempDir() returned %v", err)
	}
	notEmpty, err := ioutil.TempDir("", "notempty")
	if err != nil {
		return "", "", fmt.Errorf("ioutil.TempDir('', 'notempty') returned %v", err)
	}
	_, err = ioutil.TempFile(notEmpty, "")
	if err != nil {
		return "", "", fmt.Errorf("ioutil.TempFile(%q, '') returned %v", notEmpty, err)
	}
	return empty, notEmpty, nil
}

// fakeISO represents iso.Handler. It inherits all members of iso.Handler
// through embedding. Unimplemented members send a clear signal during tests
// because they will panic if called, allowing us to implement only the minimum
// set of members required for testing.
type fakeISO struct {
	size     uint64
	mount    string
	contents []string
	copyErr  error
}

func (f *fakeISO) Size() uint64 {
	return f.size
}

func (f *fakeISO) MountPath() string {
	return f.mount
}

func (f *fakeISO) Contents() []string {
	return f.contents
}

func (f *fakeISO) Copy(dest string) error {
	return f.copyErr
}

func (f *fakeISO) Dismount() error {
	return nil
}

// cmpPartitions is a partial custom comparer for select (all but disk)
// unexported fields of a given member of a Partition slice set by Partition()
// in Linux or Wipe() in Darwin. Errors describing fields that do not match are
// returned. When all checked fields  are equal, nil is returned.
func cmpPartitions(gotPartitions, wantPartitions []Partition, index int) error {
	if len(gotPartitions) != len(wantPartitions) {
		return fmt.Errorf("partition count mismatch, got: %d, want: %d", len(gotPartitions), len(wantPartitions))
	}
	if len(wantPartitions) == 0 {
		return nil
	}
	got := gotPartitions[index]
	want := wantPartitions[index]
	if got.id != want.id {
		return fmt.Errorf("partition index %d id mismatch, got: %q, want: %q", index, got.id, want.id)
	}
	if got.path != want.path {
		return fmt.Errorf("partition index %d path mismatch, got: %q, want: %q", index, got.path, want.path)
	}
	if got.mount != want.mount {
		return fmt.Errorf("partition index %d mount mismatch, got: %v, want: %v", index, got.mount, want.mount)
	}
	if got.label != want.label {
		return fmt.Errorf("partition index %d label mismatch, got: %q, want: %q", index, got.label, want.label)
	}
	if got.fileSystem != want.fileSystem {
		return fmt.Errorf("partition index %d fileSystem mismatch, got: %q, want: %q", index, got.fileSystem, want.fileSystem)
	}
	if got.size != want.size {
		return fmt.Errorf("partition index %d size mismatch, got: %d, want: %d", index, got.size, want.size)
	}
	return nil
}

func TestContents(t *testing.T) {
	// Temp folders representing file system contents.
	empty, notEmpty, err := fakeFileSystems()
	if err != nil {
		t.Fatalf("fakeFileSystems() returned %v", err)
	}
	defer os.RemoveAll(empty)
	defer os.RemoveAll(notEmpty)

	tests := []struct {
		desc     string
		part     Partition
		contents int
		want     error
	}{
		{
			desc: "partition not mounted",
			part: Partition{},
			want: errNotMounted,
		},
		{
			desc: "bad partition path",
			part: Partition{mount: "non_existent_path"},
			want: errInput,
		},
		{
			desc: "empty partition",
			part: Partition{mount: empty},
			want: nil,
		},
		{
			desc:     "non-empty partition",
			part:     Partition{mount: notEmpty},
			contents: 1,
			want:     nil,
		},
	}
	for _, tt := range tests {
		contents, err := tt.part.Contents()
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: contents() got = %q, want = %q", tt.desc, err, tt.want)
		}
		if len(contents) != tt.contents {
			t.Errorf("%s: unexpected contents got = %d, want = %d", tt.desc, len(contents), tt.contents)
		}
	}
}

func TestLabel(t *testing.T) {
	want := "testLabel"
	p := Partition{label: want}
	if got := p.Label(); got != want {
		t.Errorf("Label() got: %q, want: %q", got, want)
	}
}

func TestMountPoint(t *testing.T) {
	want := "some/test/path"
	p := Partition{mount: want}
	if got := p.MountPoint(); got != want {
		t.Errorf("MountPoint() got: %q, want: %q", got, want)
	}
}
