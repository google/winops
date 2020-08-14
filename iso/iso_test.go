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

package iso

import (
	"fmt"
	"regexp"
	"testing"
)

// cmpHandler examines the properties of Handler individually instead of
// collectively via, say, cmp.Diff() since one of the values of the handler is
// not deterministic.
func cmpHandler(got *Handler, want *Handler) error {
	if got == nil && want == nil {
		return nil
	}
	if got == nil && want != nil || got != nil && want == nil {
		return fmt.Errorf("got: %v, want: %v", got, want)
	}
	if want.image != got.image {
		return fmt.Errorf("got image: %q, want: %q", got.image, want.image)
	}
	if want.size != got.size {
		return fmt.Errorf("got size: %d, want %d", got.size, want.size)
	}
	if len(want.contents) > len(got.contents) {
		return fmt.Errorf("got len(contents): %d, want %d", len(got.contents), len(want.contents))
	}
	if want.mount == got.mount {
		return nil
	}
	// The actual mount path will have a random, nine-digit suffix, so we'll
	// match everything but that.
	match, err := regexp.MatchString(want.mount+"[0-9]{9}", got.mount)
	if err != nil {
		return fmt.Errorf("regexp.MatchString() returned %v", err)
	}
	if !match {
		return fmt.Errorf("got mount: %q, want: %q", got.mount, want.mount)
	}
	return nil
}

func TestImagePath(t *testing.T) {
	path := "fake/iso/file"
	want := path
	h := &Handler{image: path, mount: "fake/mount/point"}

	if got := h.ImagePath(); got != want {
		t.Errorf("ImagePath() got: %q, want: %q", got, want)
	}
}

func TestMountPath(t *testing.T) {

	path := "fake/mount/point"
	want := path
	h := &Handler{image: "fake/iso/file", mount: path}

	if got := h.MountPath(); got != want {
		t.Errorf("MountPath() got: %q, want: %q", got, want)
	}
}

func TestSize(t *testing.T) {
	size := uint64(123456789)
	want := size
	h := &Handler{size: size}

	if got := h.Size(); got != want {
		t.Errorf("Size() got: %d, want: %d", got, want)
	}
}

func TestContents(t *testing.T) {
	contents := []string{"fake/path/1", "fake/path/2"}
	want := contents
	h := Handler{contents: contents}

	// A simple length check should be sufficient for test purposes.
	if got := h.Contents(); len(got) != len(want) {
		t.Errorf("Contents() got: %v, want: %v", got, want)
	}
}
