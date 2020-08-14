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
	"io/ioutil"
	"testing"
)

func TestMount(t *testing.T) {
	isoFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tests := []struct {
		desc         string
		fakeMountCmd func(string) ([]byte, error)
		in           string
		out          *Handler
		err          error
	}{
		{
			desc:         "empty ISO file",
			fakeMountCmd: func(string) ([]byte, error) { return nil, errInput },
			in:           "",
			out:          nil,
			err:          errInput,
		},
		{
			desc:         "bad iso path",
			fakeMountCmd: func(string) ([]byte, error) { return nil, nil },
			in:           "fake/iso/file",
			out:          nil,
			err:          errFileNotFound,
		},
		{
			desc:         "failed mount",
			fakeMountCmd: func(string) ([]byte, error) { return nil, errMount },
			in:           isoFile.Name(),
			out:          nil,
			err:          errMount,
		},
		{
			desc:         "file not found",
			fakeMountCmd: func(string) ([]byte, error) { return []byte("Mount-DiskImage cannot find the file"), nil },
			in:           "/bad/fake/iso/file",
			out:          nil,
			err:          errFileNotFound,
		},
		{
			desc:         "general Mount-DiskImage error",
			fakeMountCmd: func(string) ([]byte, error) { return []byte("Mount-DiskImage failed"), nil },
			in:           isoFile.Name(),
			out:          nil,
			err:          errPSGeneral,
		},
		{
			desc:         "empty Mount-DiskImage return",
			fakeMountCmd: func(string) ([]byte, error) { return []byte(""), nil },
			in:           isoFile.Name(),
			out:          nil,
			err:          errInvalidDrive,
		},
		{
			desc:         "invalid drive spec",
			fakeMountCmd: func(string) ([]byte, error) { return []byte("DD:"), nil },
			in:           isoFile.Name(),
			out:          nil,
			err:          errInvalidDrive,
		},
		{
			desc:         "successful mount",
			fakeMountCmd: func(string) ([]byte, error) { return []byte("C:\r\n"), nil },
			in:           isoFile.Name(),
			out:          &Handler{image: isoFile.Name(), mount: `C:\`, contents: []string{"contents"}},
			err:          nil,
		},
	}

	for _, tt := range tests {
		mountCmd = tt.fakeMountCmd
		got, err := mount(tt.in)
		if chErr := cmpHandler(got, tt.out); chErr != nil {
			t.Errorf("%s: cmphandler() returned %v", tt.desc, chErr)
		}
		if err == tt.err {
			continue
		}
		if err == nil || tt.err == nil {
			t.Errorf("%s: unexpected nil encountered or test misconfigured, got err: '%v', want: '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: got mount() err: '%v', want: '%v'", tt.desc, err, tt.err)
		}
	}
}

func TestCopy(t *testing.T) {
	tests := []struct {
		desc        string
		fakeCopyCmd func(string, string) ([]byte, error)
		handler     *Handler
		dest        string
		err         error
	}{
		{
			desc:        "empty source",
			fakeCopyCmd: func(string, string) ([]byte, error) { return nil, nil },
			handler:     &Handler{mount: ""},
			dest:        "fakeDst",
			err:         errNotMounted,
		},
		{
			desc:        "empty destination",
			fakeCopyCmd: func(string, string) ([]byte, error) { return nil, nil },
			handler:     &Handler{mount: "fakeSrc"},
			dest:        "",
			err:         errInput,
		},
		{
			desc:        "copyCmd error",
			fakeCopyCmd: func(string, string) ([]byte, error) { return nil, errors.New("error") },
			handler:     &Handler{mount: "error"},
			dest:        "fakeDst",
			err:         errCopy,
		},
		{
			desc:        "Copy-Item error",
			fakeCopyCmd: func(string, string) ([]byte, error) { return []byte("Copy-Item failure"), nil },
			handler:     &Handler{mount: "fakeSrc"},
			dest:        "fakeDst",
			err:         errPSGeneral,
		},
		{
			desc:        "successful copy",
			fakeCopyCmd: func(string, string) ([]byte, error) { return nil, nil },
			handler:     &Handler{mount: "fakeSrc"},
			dest:        "fakeDst",
			err:         nil,
		},
	}
	for _, tt := range tests {
		copyCmd = tt.fakeCopyCmd
		err := tt.handler.Copy(tt.dest)
		if err == tt.err {
			continue
		}
		if err == nil || tt.err == nil {
			t.Errorf("%s: unexpected nil encountered or test misconfigured, got err: '%v', want: '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Copy() got: %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestDismount(t *testing.T) {
	// A fake temporary directory
	fakeMount, err := ioutil.TempDir("", "dismount_test_")
	if err != nil {
		t.Fatalf("could not create fake mount directory: %v", err)
	}

	tests := []struct {
		desc            string
		fakeDismountCmd func(string) ([]byte, error)
		handler         *Handler
		err             error
	}{
		{
			desc:            "empty image",
			fakeDismountCmd: func(string) ([]byte, error) { return nil, nil },
			handler:         &Handler{image: ""},
			err:             errInput,
		},
		{
			desc:            "dismountCmd error",
			fakeDismountCmd: func(string) ([]byte, error) { return nil, errors.New("error") },
			handler:         &Handler{image: "present"},
			err:             errDismount,
		},
		{
			desc:            "Dismount-DiskImage error",
			fakeDismountCmd: func(string) ([]byte, error) { return []byte("Dismount-DiskImage failure"), nil },
			handler:         &Handler{image: fakeMount},
			err:             errPSGeneral,
		},
		{
			desc:            "successful dismount",
			fakeDismountCmd: func(string) ([]byte, error) { return nil, nil },
			handler:         &Handler{image: fakeMount},
			err:             nil,
		},
	}

	for _, tt := range tests {
		dismountCmd = tt.fakeDismountCmd
		err := tt.handler.Dismount()
		if err == tt.err {
			continue
		}
		if err == nil || tt.err == nil {
			t.Errorf("%s: unexpected nil encountered or test misconfigured, got err: '%v', want: '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Dismount() got: %v, want: %v", tt.desc, err, tt.err)
		}
	}
}
