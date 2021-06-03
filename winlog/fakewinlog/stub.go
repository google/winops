// Copyright 2021 Google LLC
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

package fakewinlog

import "github.com/pkg/errors"

// retValue describes the return values used for the interface
type events struct {
	data  []string
	found bool
	done  bool
	err   error
}

// stubWindowsEvent implements simple.WindowsEvent interface, but contains some
// stubbed return values for unit tests.
type stubWindowsEvent struct {
	waiting  []events
	rendered []events
	bookmark string
	reset    error
	close    error
}

// Subscribe implements simple.WindowsEvent interface.
func (e *stubWindowsEvent) Subscribe(_ string, _ map[string]string) error {
	return nil
}

// WaitForSingleObject implements WindowsEvent interface.
func (e *stubWindowsEvent) WaitForSingleObject(timeout uint32) (bool, error) {
	if len(e.waiting) > 0 {
		w := e.waiting[0]
		e.waiting = e.waiting[1:]
		return w.found, w.err
	}
	return false, errors.New("quit now")
}

// RenderedEvents implements WindowsEvent interface.
func (e *stubWindowsEvent) RenderedEvents(_ int) ([]string, bool, error) {
	if len(e.rendered) > 0 {
		w := e.rendered[0]
		e.rendered = e.rendered[1:]
		return w.data, w.done, w.err
	}
	return nil, true, errors.New("quit now")
}

// Bookmark implements WindowsEvent interface.
func (e *stubWindowsEvent) Bookmark() (string, error) {
	if e.bookmark == "" {
		return e.bookmark, errors.New("empty bookmark")
	}
	return e.bookmark, nil
}

// ResetEvent implements WindowsEvent interface.
func (e *stubWindowsEvent) ResetEvent() error {
	return e.reset
}

// Close implements WindowsEvent interface.
func (e *stubWindowsEvent) Close() error {
	return e.close
}
