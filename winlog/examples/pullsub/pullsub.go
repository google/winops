//go:build windows
// +build windows

// Copyright 2017 Google Inc.
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

// Binary pullsub is an example application using the Windows Event Log API
// "pull" subscription model to print events to the console.
package main

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows"
	"github.com/google/winops/winlog/wevtapi"
	"github.com/google/winops/winlog"
)

func main() {
	// Initialize a subscription with defaults.
	config, err := winlog.DefaultSubscribeConfig()
	if err != nil {
		log.Fatalf("winlog.DefaultSubscribeConfig failed: %v", err)
	}
	// Get a bookmark from the registry to use with the subscription for persistence of state.
	err = winlog.GetBookmarkRegistry(config, registry.CURRENT_USER, `SOFTWARE\Logging`, "Bookmark")
	if err != nil {
		log.Fatalf("winlog.GetBookmarkRegistry failed: %v", err)
	}
	config.Flags = wevtapi.EvtSubscribeStartAfterBookmark
	subscription, err := winlog.Subscribe(config)
	if err != nil {
		log.Fatalf("winlog.Subscribe failed: %v", err)
	}
	defer winlog.Close(subscription)

	publisherCache := make(map[string]windows.Handle)
	defer func() {
		for _, h := range publisherCache {
			winlog.Close(h)
		}
	}()

	for {
		// Wait for events that match the query. Timeout in milliseconds.
		status, err := windows.WaitForSingleObject(config.SignalEvent, 10000)
		if err != nil {
			fmt.Fprintf(os.Stderr, "windows.WaitForSingleObject failed: %v", err)
			break
		}
		// Get a block of events once signaled.
		if status == syscall.WAIT_OBJECT_0 {
			// Enumerate and render available events in blocks of up to 100.
			renderedEvents, err := winlog.GetRenderedEvents(config, publisherCache, subscription, 100, 1033)
			// If no more events are available reset the subscription signal.
			if err == syscall.Errno(259) { // ERROR_NO_MORE_ITEMS
				windows.ResetEvent(config.SignalEvent)
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "winlog.GetRenderedEvents failed: %v", err)
				break
			}
			// Print the events.
			for _, event := range renderedEvents {
				fmt.Println(event)
			}
			// Persist the bookmark.
			err = winlog.SetBookmarkRegistry(config.Bookmark, registry.CURRENT_USER,
				`SOFTWARE\Logging`, "Bookmark")
			if err != nil {
				log.Fatalf("winlog.SetBookmarkRegistry failed: %v", err)
			}
		}
	}
}
