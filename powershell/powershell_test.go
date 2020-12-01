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

package powershell

import (
	"errors"
	"testing"
)

func TestCommand(t *testing.T) {
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
