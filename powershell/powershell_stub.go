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

//go:build !windows
// +build !windows

package powershell

// Command executes the provided command string in PowerShell, using the
// Command parameter. It requires a string representing the command.
// Optionally, the caller may specify a slice of strings that should be
// considered errors if present and an alternate PSConfig.
//
// This call is unsupported on non-Windows platforms.
func Command(psCmd string, supplemental []string, config *PSConfig) ([]byte, error) {
	return nil, ErrUnsupported
}

// Version gathers powershell version information from the host, returns an error if version information cannot be obtained.
//
// This call is unsupported on non-Windows platforms.
func Version() (VersionTable, error) {
	return VersionTable{}, ErrUnsupported
}
