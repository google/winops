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

// Package powershell provides simple handling for invoking PowerShell cmdlets
// on Windows systems and handling errors in a meaningful way.
package powershell

import (
	"errors"
	"fmt"
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
	// ErrUnsupported indicates an unsupported function call
	ErrUnsupported = errors.New("unsupported function call")
)

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
