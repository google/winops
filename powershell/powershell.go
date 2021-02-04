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

// Package powershell provides simple handling for invoking PowerShell cmdlets
// on Windows systems and handling errors in a meaningful way.
package powershell

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
)

// ErrorAction represents PowerShell's $ErrorActionPreference variable.
// https://devblogs.microsoft.com/scripting/hey-scripting-guy-how-can-i-use-erroractionpreference-to-control-cmdlet-handling-of-errors/
// https://docs.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_preference_variables?view=powershell-7.1
type ErrorAction string

const (
	// SilentlyContinue causes scripts to continue silently for
	// non-terminating cmdlet errors.
	SilentlyContinue ErrorAction = "SilentlyContinue"
	// Continue allows scripts to continue, but still displays errors in output.
	Continue ErrorAction = "Continue"
	// Stop causes all cmdlet errors to be terminating errors.
	Stop ErrorAction = "Stop"
	// Inquire causes a debug message to be displayed and asks if the user wants
	// to continue.
	Inquire ErrorAction = "Inquire"
)

// PSConfig represents the command line parameters to use with invocations.
type PSConfig struct {
	ErrAction ErrorAction // The ErrorAction that is set.
	Params    []string    // Additional parameters for calls to powershell.exe
}

var (
	// defaultConfig is the default configuration for PowerShell calls.
	// It assumes that powershell.exe is calledc with -NoProfile and that
	// the desired ErrorAction preference is "Stop"
	defaultConfig = PSConfig{
		ErrAction: Stop,
		Params:    []string{"-NoProfile"},
	}

	// ErrPowerShell represents an error returned by PowerShell.
	ErrPowerShell = errors.New("powershell error")
	// ErrSupplemental represents output that contians a supplemental error.
	ErrSupplemental = errors.New("supplemental error")
	errCompile      = errors.New("compile error")

	// Dependency injection for testing.
	powerShellCmd = powerShell
)

// powerShell represents the OS command used to run a powerShell cmdlet on
// Windows. The raw output is provided to the caller for error handling.
// The params parameter should be populated with all of the required
// parameters to invoke powershell.exe from the command line. If an error is
// returne to the OS, it will be returned here.
func powerShell(params []string) ([]byte, error) {
	out, err := exec.Command("powershell.exe", params...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command("powershell.exe", %s) command returned: %q: %v`, params, out, err)
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

// supplementalErr checks the PowerShell output to determine if it contains
// any user specified error strings.
func supplementalErr(output []byte, supplemental []string) error {
	for _, supplement := range supplemental {
		expression := fmt.Sprintf(`(?i)%s[\s\S]+`, supplement)
		regex, err := regexp.Compile(expression)
		if err != nil {
			return fmt.Errorf("regexp.Compile(%q) returned %v: %w", expression, err, errCompile)
		}
		if regex.Match(output) {
			return fmt.Errorf("output %s contains supplemental error text %q: %w", output, supplement, ErrPowerShell)
		}
	}
	// No matches found, output contains no errors.
	return nil
}

// VersionDetail is structured to hold version information as output by powershell components
type VersionDetail struct {
	Major         int
	Minor         int
	Build         int
	Revision      int
	MajorRevision int
	MinorRevision int
}

// VersionTable Contains the full output of $PSVersionTable from powershell
type VersionTable struct {
	PSVersion                 VersionDetail
	PSEdition                 string
	PSCompatibleVersions      []VersionDetail
	BuildVersion              VersionDetail
	CLRVersion                VersionDetail
	WSManStackVersion         VersionDetail
	PSRemotingProtocolVersion VersionDetail
	SerializationVersion      VersionDetail
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
