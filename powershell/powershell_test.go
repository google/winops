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

package powershell

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	testData = "testdata/"
)

func init() {
	Command("Set-ExecutionPolicy Bypass", nil, nil)
}

func TestCommand(t *testing.T) {
	defer func() {
		powerShellCmd = powerShell
	}()
	tests := []struct {
		desc              string
		psCmd             string
		supplemental      []string
		fakePowerShellCmd func([]string) ([]byte, error)
		err               error
	}{
		{
			desc:              "powershell error",
			psCmd:             "Verb-Noun",
			fakePowerShellCmd: func([]string) ([]byte, error) { return nil, errors.New("powershell-error") },
			err:               ErrPowerShell,
		},
		{
			desc:              "supplemental error",
			psCmd:             "Verb-Noun",
			supplemental:      []string{"cmdlet-error"},
			fakePowerShellCmd: func([]string) ([]byte, error) { return []byte("cmdlet-error: details"), nil },
			err:               ErrSupplemental,
		},
		{
			desc:              "success",
			psCmd:             "Verb-Noun",
			fakePowerShellCmd: func([]string) ([]byte, error) { return []byte("output"), nil },
			err:               nil,
		},
	}

	for _, tt := range tests {
		powerShellCmd = tt.fakePowerShellCmd
		if _, err := Command(tt.psCmd, tt.supplemental, nil); !errors.Is(err, tt.err) {
			t.Errorf("%s: Command(%q, %v, nil) = %v, want: %v", tt.desc, tt.psCmd, tt.supplemental, err, tt.err)
		}
	}
}

func TestFile(t *testing.T) {
	tests := []struct {
		desc         string
		psFile       string
		psArgs       []string
		supplemental []string
		wantOut      []byte
		err          error
	}{
		{
			desc:    "execute ok; no output",
			psFile:  filepath.Join(testData + "hello.ps1"),
			err:     nil,
			wantOut: []byte{},
		},
		{
			desc:    "execute ok; with output",
			psFile:  filepath.Join(testData + "hello.ps1"),
			psArgs:  []string{"-Verbose"},
			err:     nil,
			wantOut: []byte("VERBOSE: hello world\n"),
		},
		{
			desc:    "input file missing",
			psFile:  filepath.Join(testData + "missing.ps1"),
			err:     ErrPowerShell,
			wantOut: []byte{},
		},
	}

	for _, tt := range tests {
		out, err := File(tt.psFile, tt.psArgs, tt.supplemental, nil)
		if !errors.Is(err, tt.err) {
			t.Errorf("%s: File(%q, %v, %v, nil) produced unexpected error %v, want: %v", tt.desc, tt.psFile, tt.psArgs, tt.supplemental, err, tt.err)
			continue
		}
		if diff := cmp.Diff(out, tt.wantOut); diff != "" {
			t.Errorf("%s: File(%q, %v, %v, nil) = %s, want: %s", tt.desc, tt.psFile, tt.psArgs, tt.supplemental, out, tt.wantOut)
		}
	}
}

func TestNewSession(t *testing.T) {
	ps, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() failed: %v", err)
	}
	if err := ps.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
}

func TestExecute(t *testing.T) {
	ps, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() failed: %v", err)
	}
	defer ps.Close()
	_, err = ps.Execute("ipconfig")
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
}
