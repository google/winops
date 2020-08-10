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

package iso

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// Wrapped errors for testing.
	errPSGeneral    = errors.New("powershell error")
	errInvalidDrive = errors.New("invalid drive specification")

	// Regex for powershell error handling
	regExFileNotFound  = regexp.MustCompile(`Mount-DiskImage[\s\S]+cannot find the file`)
	regExPSMountErr    = regexp.MustCompile(`Mount-DiskImage[\s\S]+`)
	regExDriveSpec     = regexp.MustCompile(`^[A-Za-z]:+`)
	regExPSCopyErr     = regexp.MustCompile(`Copy-Item[\s\S]+`)
	regExPSDismountErr = regexp.MustCompile(`Dismount-DiskImage[\s\S]+`)
)

// mntCmd represents the PowerShell command used to mount an ISO
// on windows. The raw output is provided to the caller for handling.
// An error is returned if powershell.exe returns it, but if
// a powershell cmdlet throws an error this does not cause powershell.exe
// to return an error. Thus, output issues and cmdlet errors should be
// handled by the caller.
func mntCmd(isoFile string) ([]byte, error) {
	psBlock := fmt.Sprintf(`& { (Mount-DiskImage -ImagePath %s -StorageType ISO -PassThru | Get-Volume | select -ExpandProperty DriveLetter) + ':' }`, isoFile)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", psBlock).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("powershell.exe", "-NoProfile", "-Command", %s) command returned: %q: %v`, psBlock, out, err)
	}
	return out, nil
}

// mountWindows mounts the ISO at the given path to a temporary directory and
// returns a pointer to a Handler representing the mounted image. It is used
// only on Windows.
func mount(isoFile string) (*Handler, error) {
	if isoFile == "" {
		return nil, fmt.Errorf("mounting %q failed with: %w", isoFile, errInput)
	}
	file, err := os.Stat(isoFile)
	if err != nil {
		return nil, fmt.Errorf("os.Stat(%q) returned %v: %w", isoFile, err, errFileNotFound)
	}
	out, err := mountCmd(isoFile)
	if err != nil {
		return nil, fmt.Errorf("mountCmd(%q) returned %v: %w", isoFile, err, errMount)
	}
	if regExFileNotFound.Match(out) {
		return nil, fmt.Errorf("Mount-DiskImage returned %q: %w", out, errFileNotFound)
	}
	if regExPSMountErr.Match(out) {
		return nil, fmt.Errorf("Mount-DiskImage returned %q: %w", out, errPSGeneral)
	}
	// Mount-DiskImage returns drive letter + colon, e.g., "D:" when successful
	if !regExDriveSpec.Match(out) {
		return nil, fmt.Errorf("%q is an: %w", out, errInvalidDrive)
	}
	mntDrive := string(out[0:2])

	contents, err := contents(mntDrive)
	if err != nil {
		return nil, fmt.Errorf("contents(%s) returned %w", mntDrive+":", err)
	}
	return &Handler{
		image:    isoFile,
		mount:    mntDrive + `\`,
		size:     uint64(file.Size()),
		contents: contents,
	}, nil
}

// cpyCmd represents the OS command used to copy files on
// Windows. The raw output is only required for error handling.
// Similar to mount, error is not an indicator of cmdlet
// success or failure, therefore output should be checked
// for error strings by the caller.
func cpyCmd(src, dst string) ([]byte, error) {
	psBlock := fmt.Sprintf(`& { Copy-Item -Path %s\* -Destination %s -Recurse }`, src, dst)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", psBlock).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("powershell.exe", "-NoProfile", "-Command", %s) returned: %q: %v`, psBlock, out, err)
	}
	return out, nil
}

// Copy Recursively copies the contents of a mounted ISO to
// a destination folder. Additional handling for the output
// is present on Windows to handle powershell cmdlet errors.
func (iso *Handler) Copy(dst string) error {
	if iso.mount == "" {
		return fmt.Errorf("source was empty: %w", errNotMounted)
	}
	if dst == "" {
		return fmt.Errorf("destination was empty: %w", errInput)
	}
	// If the path doesn't contain a colon, we may have a naked drive letter
	// which is a problem for Copy-Item.
	if !strings.Contains(dst, ":") {
		dst = dst + ":"
	}
	out, err := copyCmd(iso.mount, dst)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errCopy)
	}
	if regExPSCopyErr.Match(out) {
		return fmt.Errorf("%v powershell returned %q: %w", errCopy, out, errPSGeneral)
	}
	return nil
}

// dmCmd represents the eject command used to dismount a
// previously mounted ISO on Windows. Powershell cmdlet errors
// should be handled by the caller.
func dmCmd(image string) ([]byte, error) {
	psBlock := fmt.Sprintf(`& { Dismount-DiskImage -ImagePath '%s' }`, image)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", psBlock).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("powershell.exe", "-NoProfile", "-Command", %s) returned: %q: %v`, psBlock, out, err)
	}
	return out, nil
}

// Dismount dismounts an ISO and its associated drive letter.
func (iso *Handler) Dismount() error {
	if iso.image == "" {
		return fmt.Errorf("image path was empty: %w", errInput)
	}
	out, err := dismountCmd(iso.image)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errDismount)
	}
	if regExPSDismountErr.Match(out) {
		return fmt.Errorf("%v, powershell returned %q: %w", errDismount, out, errPSGeneral)
	}
	return nil
}
