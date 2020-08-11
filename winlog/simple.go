// Package simple provides a simple interface to communicate with Windows Event Log, but hides any
// Windows specific names/structs. So, we can have different implementation on different platforms
// and mock Windows APIs for unit test.
package simple

import "time"

// Event defines a set of APIs to get events from Windows Event Log.
type Event interface {
	// Subscribe initializes a subscription for Windows Event Log. Close must be called when
	// finished.
	Subscribe(bookmark string, query map[string]string) error
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
