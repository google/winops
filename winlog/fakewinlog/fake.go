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

import (
	"encoding/gob"
	"errors"
	"strings"
	"time"

	"github.com/google/winops/winlog/simple"
)

// FakeWindowsAPI provides a fake implementation of winlog's simple.Event
// interface. It is limited in the following ways:
//
// 1) Only query keys are used, and values are ignored (assumed to be "*").
//
// 2) All available events must be held in memory, in the Events field.
//
// 3) There is no thread-safety whatsoever.
type FakeWindowsAPI struct {
	Events         map[string][]string
	cursors        map[string]int
	query          map[string]string
	eventSignalled bool
}

var _ simple.Event = (*FakeWindowsAPI)(nil)

// Subscribe initializes a subscription for Windows Event Log. Close must be called when
// finished.
func (w *FakeWindowsAPI) Subscribe(bookmark string, query map[string]string) error {
	w.query = query
	w.cursors = map[string]int{}
	for k, _ := range query {
		if len(w.Events[k]) != 0 {
			w.eventSignalled = true
			break
		}
	}
	if bookmark != "" {
		d := gob.NewDecoder(strings.NewReader(bookmark))
		return d.Decode(&w.cursors)
	}
	return nil
}

// WaitForSingleObject simulates waiting for events to be available.
func (w *FakeWindowsAPI) WaitForSingleObject(timeout time.Duration) (bool, error) {
	if w.eventSignalled {
		return true, nil
	}

	time.Sleep(timeout)
	return false, nil
}

// RenderedEvents returns up to `max` events from w.Events, if they match the
// query provided to Subscribe. If no further events are available after this
// call, the boolean return value will be set to true.
func (w *FakeWindowsAPI) RenderedEvents(max int) (events []string, done bool, err error) {
	if w.query == nil || w.cursors == nil {
		return nil, true, errors.New("not open")
	}

	var ret []string
	for k, s := range w.Events {
		if _, ok := w.query[k]; !ok {
			continue
		}

		available := s[w.cursors[k]:]

		if lim := (max - len(ret)); len(available) > lim {
			available = available[:lim]
		}

		ret = append(ret, available...)
		w.cursors[k] += len(available)

		if len(ret) == max {
			return ret, false, nil
		}
	}

	return ret, true, nil
}

// Bookmark returns a bookmark value that can be passed to Subscribe to resume
// consuming Events in the future.
func (w *FakeWindowsAPI) Bookmark() (string, error) {
	var b strings.Builder
	e := gob.NewEncoder(&b)
	if err := e.Encode(w.cursors); err != nil {
		return "", err
	}
	return b.String(), nil
}

// ResetEvent does nothing, unsuccessfully.
func (w *FakeWindowsAPI) ResetEvent() error {
	w.eventSignalled = false
	return nil
}

// Close closes the subscription.
func (w *FakeWindowsAPI) Close() error {
	w.query = nil
	w.cursors = nil
	return nil
}

// Append adds more events to w.Events.
func (w *FakeWindowsAPI) Append(events map[string][]string) {
	for k, s := range events {
		w.Events[k] = append(w.Events[k], s...)
	}
	w.eventSignalled = true
}
