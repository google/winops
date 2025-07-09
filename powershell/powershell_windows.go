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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
// returned to the OS, it will be returned here.
func powerShell(params []string) ([]byte, error) {
	out, err := exec.Command(powerShellExe, params...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf(`exec.Command(%q, %s) command returned: %q: %v`, powerShellExe, params, out, err)
	}
	return out, nil
}

func execute(params []string, supplemental []string) ([]byte, error) {
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

	return execute(params, supplemental)
}

// File executes a PowerShell script file.
func File(path string, args []string, supplemental []string, config *PSConfig) ([]byte, error) {
	// Apply the default PSConfig if none was specified.
	if config == nil {
		c := defaultConfig
		config = &c

	}
	params := append(config.Params, "-File", path)
	params = append(params, args...)

	return execute(params, supplemental)
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

// Session manages a persistent PowerShell process.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
}

// NewSession creates and starts a new PowerShell session.
func NewSession() (*Session, error) {
	cmd := exec.Command(powerShellExe, "-NoExit", "-NoProfile", "-Command", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start PowerShell session: %w", err)
	}
	return &Session{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Execute runs a command in the PowerShell session and returns its output.
func (ps *Session) Execute(command string) (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// This is annoying but we need some way of identifying the end of the command's output.
	const endMarker = "----------END-OF-COMMAND-OUTPUT----------"
	fullCommand := fmt.Sprintf("%s; Write-Host '%s'\n", command, endMarker)
	if _, err := ps.stdin.Write([]byte(fullCommand)); err != nil {
		return "", fmt.Errorf("failed to write to stdin: %w", err)
	}

	var output strings.Builder
	for {
		line, err := ps.stdout.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read from stdout: %w", err)
		}
		if strings.TrimSpace(line) == endMarker {
			break
		}
		output.WriteString(line)
	}
	return strings.TrimSpace(output.String()), nil
}

// Close terminates the PowerShell session.
func (ps *Session) Close() error {
	// Attempt to close the stdin pipe first.
	if err := ps.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	return ps.cmd.Process.Kill()
}
