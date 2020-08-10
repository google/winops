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

// Package iso provides uniform cross-platform handling for ISO files.
package iso

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

var (
	// Wrapped errors for testing.
	errInput        = errors.New("invalid or missing input")
	errFileNotFound = errors.New("file not found")
	errMount        = errors.New("error during mounting")
	errNotMounted   = errors.New("iso not mounted")
	errCopy         = errors.New("error during file copy")
	errDismount     = errors.New("error dismounting iso")
	errDmCleanup    = errors.New("error cleaning up mount dir")

	// Dependency injections.
	mountCmd    = mntCmd
	copyCmd     = cpyCmd
	dismountCmd = dmCmd
)

// Handler exposes a uniform method for ISO handling on all platforms.
type Handler struct {
	image    string
	mount    string
	size     uint64
	contents []string
}

// Mount mounts the ISO at the given path and returns a platform-specific
// handler. It is the primary entry point for the ISO package.
func Mount(isoFile string) (*Handler, error) {
	return mount(isoFile)
}

// ImagePath returns the path to the ISO image used when creating the
// ISO handler.
func (iso *Handler) ImagePath() string {
	return iso.image
}

// MountPath returns the path to the directory where the ISO is mounted.
func (iso *Handler) MountPath() string {
	return iso.mount
}

// Contents returns the contents of the ISO as a slice of strings.
func (iso *Handler) Contents() []string {
	return iso.contents
}

// Size returns the size of the ISO that is mounted in bytes.
func (iso *Handler) Size() uint64 {
	return iso.size
}

// contents returns a list of the contents of a directory.
func contents(path string) ([]string, error) {
	if path == "" {
		return []string{}, errInput
	}
	// Enumerate the contents.
	list, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadDir(%s) returned %v: %w", path, err, errFileNotFound)
	}
	var contents []string
	for _, f := range list {
		// Construct complete dir/dest paths
		fullPath := filepath.Join(path, f.Name())
		contents = append(contents, fullPath)
	}
	return contents, nil
}
