// Copyright 2022 Google LLC
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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	// Dependency injection for testing.
	powerShellCmd = powerShell

	powerShellExe = filepath.Join(os.Getenv("SystemRoot"), `\System32\WindowsPowerShell\v1.0\powershell.exe`)
)

// powerShell represents the OS command used to run a powerShell cmdlet on
// Windows. The raw output is provided to the caller for error handling.
// The params parameter should be populated with all of the required
// parameters to invoke powershell.exe from the command line. If an error is
// returne to the OS, it will be returned here.
func powerShell(params []string) ([]byte, error) {
	out, err := exec.Command(powerShellExe, params...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command(%q, %s) command returned: %q: %v`, powerShellExe, params, out, err)
	}
	return out, nil
}

// Command executes the provided command string in PowerShell, using the
// Command parameter. It requires a string representing the command.
// Optionally, the caller may specify a slice of strings that should be
// considered errors if present and an alternate PSConfig.
//
// Example psCmd: Get-Volume D | select FileSystemLabel | ConvertTo-Json
// Documentation: https://docs.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_powershell_exe?view=powershell-5.1#-command
func Command(psCmd string, supplemental []string, config *PSConfig) ([]byte, error) {
	// Apply the default PSConfig if none was specified.
	if config == nil {
		c := defaultConfig
		config = &c

	}
	// Embed ErrorActionPreference and generate parameters.
	cmd := fmt.Sprintf(`$ErrorActionPreference="%s"; %s`, config.ErrAction, psCmd)
	params := append(config.Params, "-Command", cmd)

	// Invoke PowerShell
	out, err := powerShellCmd(params)
	if err != nil {
		return out, fmt.Errorf("powershell returned %v: %w", err, ErrPowerShell)
	}

	// Determine if the output contains a supplemental error.
	if err := supplementalErr(out, supplemental); err != nil {
		return out, fmt.Errorf("supplementalErr returned %v: %w", err, ErrSupplemental)
	}

	// Return successful output.
	return out, nil
}

// Version gathers powershell version information from the host, returns an error if version information cannot be obtained.
func Version() (VersionTable, error) {
	var psv VersionTable
	o, err := Command(`$PSVersionTable | ConvertTo-Json`, []string{}, nil)
	if err != nil {
		return psv, err
	}
	err = json.Unmarshal(o, &psv)
	return psv, err
}
