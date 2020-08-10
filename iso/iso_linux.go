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

// +build linux

package iso

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

// mntCmd represents the OS command used to mount an ISO on
// linux. The raw output is only required for error handling.
// If the mount command exits with an error it is returned.
func mntCmd(isoFile, mntDir string) error {
	args := []string{"mount", "--options", "ro,loop", isoFile, mntDir}
	if out, err := exec.Command("sudo", args...).CombinedOutput(); err != nil {
		return fmt.Errorf(`exec.Command("sudo", %q) returned %q: %v`, args, out, err)
	}
	return nil
}

// mount mounts the ISO at the given path to a temporary directory and
// returns a pointer to a Handler representing the mounted image.
func mount(isoFile string) (*Handler, error) {
	if isoFile == "" {
		return nil, fmt.Errorf("mount path was empty: %w", errInput)
	}
	file, err := os.Stat(isoFile)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, errFileNotFound)
	}
	mntDir, err := ioutil.TempDir("", "iso_mnt_")
	if err != nil {
		return nil, fmt.Errorf("ioutil.TempDir returned %v", err)
	}
	if err := mountCmd(isoFile, mntDir); err != nil {
		return nil, fmt.Errorf("%v: %w", err, errMount)
	}
	contents, err := contents(mntDir)
	if err != nil {
		return nil, err
	}
	return &Handler{
		image:    isoFile,
		mount:    mntDir,
		size:     uint64(file.Size()),
		contents: contents,
	}, nil
}

// cpyCmd represents the OS command used to copy files on
// linux. The raw output is only required for error handling.
// If the copy command exits with an error, it is returned.
func cpyCmd(src, dst string) error {
	args := "cp -Rf " + src + "/* " + dst
	if out, err := exec.Command("sudo", "/bin/sh", "-c", args).CombinedOutput(); err != nil {
		return fmt.Errorf(`exec.Command("sudo", "/bin/sh", "-c", %q) returned %q: %v`, args, out, err)
	}
	return nil
}

// Copy recursively copies the contents of a mounted ISO to a destination
// folder.
func (iso *Handler) Copy(dst string) error {
	if iso.mount == "" {
		return fmt.Errorf("source was empty: %w", errNotMounted)
	}
	if dst == "" {
		return fmt.Errorf("destination was empty: %w", errInput)
	}
	if err := copyCmd(iso.mount, dst); err != nil {
		return fmt.Errorf("%v: %w", err, errCopy)
	}
	return nil
}

// dmCmd represents the eject command used to dismount a
// previously mounted ISO on linux. The raw output is only
// required for error handling. If the command returns an
// error, it is returned.
func dmCmd(mnt string) error {
	if result, err := exec.Command("sudo", "umount", mnt).CombinedOutput(); err != nil {
		return fmt.Errorf(`exec.Command("sudo", "umount", %q) returned: %q: %v`, mnt, result, err)
	}
	return nil
}

// Dismount dismounts an ISO and removes its temporary directory.
func (iso *Handler) Dismount() error {
	if iso.mount == "" {
		return fmt.Errorf("mountpoint was empty: %w", errInput)
	}
	if err := dismountCmd(iso.mount); err != nil {
		return fmt.Errorf("%v: %w", err, errDismount)
	}
	if err := os.Remove(iso.mount); err != nil {
		return fmt.Errorf("removal of %q returned %v: %w", iso.mount, err, errDmCleanup)
	}
	return nil
}
