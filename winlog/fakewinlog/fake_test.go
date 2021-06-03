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
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/google/winops/winlog/simple"
)

func TestRenderedEvents(t *testing.T) {
	subscriber, err := setupSubscriber(
		map[string][]string{"foo": []string{"foo1", "foo2", "foo3"}},
		nil,
		map[string]string{"foo": "*"})
	if err != nil {
		t.Fatal(err)
	}

	diff, err := diffRenderedEvents(subscriber, 2, []string{"foo1", "foo2"})
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf(diff)
	}
	diff, err = diffRenderedEvents(subscriber, 1, []string{"foo3"})
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Error(diff)
	}
}

func TestMultipleChannels(t *testing.T) {
	subscriber, err := setupSubscriber(
		map[string][]string{
			"foo": []string{"foo1", "foo2", "foo3"},
			"bar": []string{"bar1", "bar2", "bar3"},
			"baz": []string{"baz1", "baz2", "baz3"},
		},
		nil,
		map[string]string{"foo": "*", "bar": "*"})
	if err != nil {
		t.Fatal(err)
	}

	diff, err := diffRenderedEvents(subscriber, 6, []string{
		"bar1", "bar2", "bar3", "foo1", "foo2", "foo3"})
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Error(diff)
	}
}

func TestBookmarkCloseReopen(t *testing.T) {
	query := map[string]string{"foo": "*", "bar": "*"}
	subscriber, err := setupSubscriber(
		map[string][]string{
			"foo": []string{"foo1", "foo2", "foo3"},
			"bar": []string{"bar1", "bar2", "bar3"},
			"baz": []string{"baz1", "baz2", "baz3"},
		},
		map[string]int{"foo": 1}, query)
	if err != nil {
		t.Fatal(err)
	}

	events, _, err := subscriber.RenderedEvents(3)
	if err != nil {
		t.Fatal(err)
	}

	diffEvents(events, []string{"bar1", "foo2", "foo3"})

	bm, err := subscriber.Bookmark()
	if err != nil {
		t.Fatal(err)
	}

	if err := subscriber.Close(); err != nil {
		t.Fatal(err)
	}

	if err := subscriber.Subscribe(bm, query); err != nil {
		t.Fatal(err)
	}

	events, _, err = subscriber.RenderedEvents(3)
	if err != nil {
		t.Fatal(err)
	}
	diffEvents(events, []string{"bar2", "bar3"})
}

func TestWaitForSingleObject(t *testing.T) {
	subscriber := &FakeWindowsAPI{Events: map[string][]string{}}
	subscriber.Subscribe("", map[string]string{"foo": "*"})

	// Before we have events, should return false.
	if ok, err := subscriber.WaitForSingleObject(0); ok || err != nil {
		t.Errorf("After no events, WaitForSingleObject(0) returned true, %v want false, nil", err)
	}

	// Append an event.
	subscriber.Append(map[string][]string{"foo": []string{"foo1"}})

	// Now the event should be triggered.
	if ok, err := subscriber.WaitForSingleObject(0); !ok || err != nil {
		t.Errorf("After single event, WaitForSingleObject(0) returned false, %v want true, nil", err)
	}

	// Read out the event.
	events, done, err := subscriber.RenderedEvents(1)
	if diff := cmp.Diff(events, []string{"foo1"}); diff != "" {
		t.Errorf("events (-) actual vs (+) expected:\n%s", diff)
	}
	if done {
		t.Errorf("RenderedEvents returned done = true, want false")
	}
	if err != nil {
		t.Errorf("RenderedEvents returned err = %v, want nil", err)
	}

	// Haven't reset, event should remain triggered even though the log is empty.
	if ok, err := subscriber.WaitForSingleObject(0); !ok || err != nil {
		t.Errorf("After reading event, WaitForSingleObject(0) returned false, %v want true, nil", err)
	}

	subscriber.ResetEvent()

	// Now we should be back at the start
	if ok, err := subscriber.WaitForSingleObject(0); ok || err != nil {
		t.Errorf("After resetting event, WaitForSingleObject(0) returned true, %v want false, nil", err)
	}
}

func TestWaitForSingleObjectInitialState(t *testing.T) {
	subscriber := &FakeWindowsAPI{Events: map[string][]string{"foo": []string{"foo1"}}}
	subscriber.Subscribe("", map[string]string{"foo": "*"})

	// This subscriber should start off with the event initially in the
	// triggered state
	if ok, err := subscriber.WaitForSingleObject(0); !ok || err != nil {
		t.Errorf("After single event, WaitForSingleObject(0) returned false, %v want true, nil", err)
	}
}

func setupSubscriber(events map[string][]string, bookmark map[string]int, query map[string]string) (*FakeWindowsAPI, error) {
	subscriber := &FakeWindowsAPI{Events: events}
	bm, err := serializeBookmark(bookmark)
	if err != nil {
		return nil, errors.Wrap(err, "serializeBookmark")
	}

	if err := subscriber.Subscribe(bm, query); err != nil {
		return nil, errors.Wrap(err, "Subscribe")
	}

	return subscriber, nil
}

func diffRenderedEvents(subscriber simple.Event, batchSize int, expected []string) (string, error) {
	events, err := renderAll(subscriber, batchSize)
	if err != nil {
		return "", errors.Wrap(err, "renderAll")
	}

	return diffEvents(events, expected), nil
}

func diffEvents(actual, expected []string) string {
	diff := cmp.Diff(actual, expected, cmpopts.SortSlices(func(x, y string) bool { return x < y }))
	if diff != "" {
		return fmt.Sprintf("events (-) actual vs (+) expected:\n%s", diff)
	}
	return ""
}

func serializeBookmark(bookmark map[string]int) (string, error) {
	if bookmark == nil {
		return "", nil
	}
	var b strings.Builder
	e := gob.NewEncoder(&b)
	if err := e.Encode(bookmark); err != nil {
		return "", errors.Wrapf(err, "could not encode bookmark %v", bookmark)
	}
	return b.String(), nil
}

func renderAll(subscriber simple.Event, batchSize int) ([]string, error) {
	var events []string
	for {
		s, cont, err := subscriber.RenderedEvents(batchSize)
		if err != nil {
			return nil, errors.Wrap(err, "RenderedEvents")
		}

		events = append(events, s...)

		if !cont {
			return events, nil
		}

		if len(s) == 0 {
			return nil, io.ErrUnexpectedEOF
		}
	}
}
