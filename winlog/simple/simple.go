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

// Package simple provides a simple interface to communicate with Windows Event Log, but hides any
// Windows specific names/structs. So, we can have different implementation on different platforms
// and mock Windows APIs for unit test.
package simple

import "time"

// Additional subscription flags for [SubscribeOptions.Flags], matching
// EVT_SUBSCRIBE_FLAGS in the Windows Event Log API.
const (
	EvtSubscribeTolerateQueryErrors = 0x1000
	EvtSubscribeStrict              = 0x10000
)

// SubscribeOptions configures a Windows Event Log subscription.
type SubscribeOptions struct {
	// Bookmark is an XML-formatted bookmark string to resume the subscription
	// from. If empty or invalid, a new empty bookmark is created and the
	// subscription starts from the oldest record.
	Bookmark string
	// Query maps Windows Event Log channel names to XPath queries. If empty,
	// the default query from winlog.DefaultSubscribeConfig (all events from
	// Application, Security, and System) is used.
	Query map[string]string
	// Flags are additional EVT_SUBSCRIBE_FLAGS (such as
	// EvtSubscribeTolerateQueryErrors) ORed with
	// EvtSubscribeStartAfterBookmark when calling EvtSubscribe.
	Flags uint32
}

// Event defines a set of APIs to get events from Windows Event Log.
type Event interface {
	// Subscribe initializes a subscription for Windows Event Log. Close must be called when
	// finished.
	Subscribe(bookmark string, query map[string]string) error
	// SubscribeWithOptions initializes a subscription for Windows Event Log with the given
	// options. Close must be called when finished.
	SubscribeWithOptions(opts SubscribeOptions) error
	// WaitForSingleObject waits for new events to arrive. Returns true if the event
	// was signalled before the timeout expired. Timeout must be between 0 and 2^32us.
	WaitForSingleObject(timeout time.Duration) (bool, error)
	// RenderedEvents returns the rendered events as a slice of UTF8 formatted XML strings.
	RenderedEvents(max int) ([]string, bool, error)
	// Bookmark returns the bookmark in XML format.
	Bookmark() (string, error)
	// ResetEvent resets the event signal after read all events to wait for new event to
	// arrive.
	ResetEvent() error
	// Close closes the subscription.
	Close() error
}
