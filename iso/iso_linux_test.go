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
	"errors"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMount(t *testing.T) {
	isoMntPnt := path.Join(os.TempDir(), "iso_mnt_")
	isoFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tests := []struct {
		desc         string
		fakeMountCmd func(string, string) error
		in           string
		out          *Handler
		err          error
	}{
		{
			desc:         "empty ISO file",
			fakeMountCmd: func(string, string) error { return nil },
			in:           "",
			out:          nil,
			err:          errInput,
		},
		{
			desc:         "bad iso path",
			fakeMountCmd: func(string, string) error { return nil },
			in:           "fake/iso/file",
			out:          nil,
			err:          errFileNotFound,
		},
		{
			desc:         "failed mount",
			fakeMountCmd: func(string, string) error { return errors.New("error") },
			in:           isoFile.Name(),
			out:          nil,
			err:          errMount,
		},
		{
			desc:         "successful mount",
			fakeMountCmd: func(string, string) error { return nil },
			in:           isoFile.Name(),
			out:          &Handler{image: isoFile.Name(), mount: isoMntPnt},
			err:          nil,
		},
	}
	for _, tt := range tests {
		mountCmd = tt.fakeMountCmd
		got, err := mount(tt.in)
		if chErr := cmpHandler(got, tt.out); chErr != nil {
			t.Errorf("%s: %v", tt.desc, chErr)
		}
		if err == tt.err {
			continue
		}
		if err == nil || tt.err == nil {
			t.Errorf("%s: unexpected nil encountered or test misconfigured, err '%v', tt.err '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: mount() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}

func TestCopy(t *testing.T) {
	tests := []struct {
		desc        string
		fakeCopyCmd func(string, string) error
		handler     *Handler
		dest        string
		err         error
	}{
		{
			desc:        "empty source",
			fakeCopyCmd: func(string, string) error { return nil },
			handler:     &Handler{mount: ""},
			dest:        "fakeDst",
			err:         errNotMounted,
		},
		{
			desc:        "empty destination",
			fakeCopyCmd: func(string, string) error { return nil },
			handler:     &Handler{mount: "fakeSrc"},
			dest:        "",
			err:         errInput,
		},
		{
			desc:        "error from copyCmd",
			fakeCopyCmd: func(string, string) error { return errors.New("error") },
			handler:     &Handler{mount: "error"},
			dest:        "fakeDst",
			err:         errCopy,
		},
		{
			desc:        "successful copy",
			fakeCopyCmd: func(string, string) error { return nil },
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
			t.Errorf("%s: unexpected nil encountered or test misconfigured, err '%v', tt.err '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Copy() err = %v, want: %v", tt.desc, err, tt.err)
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
		fakeDismountCmd func(string) error
		handler         *Handler
		err             error
	}{
		{
			desc:            "empty mountpoint",
			fakeDismountCmd: func(string) error { return nil },
			handler:         &Handler{mount: ""},
			err:             errInput,
		},
		{
			desc:            "error from dismountCmd ",
			fakeDismountCmd: func(string) error { return errors.New("error") },
			handler:         &Handler{mount: "error"},
			err:             errDismount,
		},
		{
			desc:            "cleanup failure",
			fakeDismountCmd: func(string) error { return nil },
			handler:         &Handler{mount: "non-existent"},
			err:             errDmCleanup,
		},
		{
			desc:            "successful dismount",
			fakeDismountCmd: func(string) error { return nil },
			handler:         &Handler{mount: fakeMount},
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
			t.Errorf("%s: unexpected nil encountered or test misconfigured, err '%v', tt.err '%v'", tt.desc, err, tt.err)
		}
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: Dismount() err = %v, want: %v", tt.desc, err, tt.err)
		}
	}
}
