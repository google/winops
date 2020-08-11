// +build windows

package winlog

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"github.com/google/winops/winlog/wevtapi"
)

// AvailablePublishers returns a slice of publishers registered on the system.
func AvailablePublishers() ([]string, error) {
	h, err := wevtapi.EvtOpenPublisherEnum(localMachine, mustBeZero)
	if err != nil {
		return nil, fmt.Errorf("wevtapi.EvtOpenPublisherEnum failed: %v", err)
	}
	defer Close(h)

	var publishers []string
	buf := make([]uint16, 1)
	for {
		var bufferUsed uint32
		err := wevtapi.EvtNextPublisherId(h, uint32(len(buf)), &buf[0], &bufferUsed)
		switch err {
		case nil:
			publishers = append(publishers, syscall.UTF16ToString(buf[:bufferUsed]))
		case syscall.ERROR_INSUFFICIENT_BUFFER:
			// Grow buffer.
			buf = make([]uint16, bufferUsed)
			continue
		case windows.ERROR_NO_MORE_ITEMS:
			return publishers, nil
		default:
			return nil, fmt.Errorf("wevtapi.EvtNextPublisherId failed: %v", err)
		}
	}
}

// OpenPublisherMetadata opens a handle to the publisher's metadata.
// Close must be called on the returned handle when finished.
func OpenPublisherMetadata(session windows.Handle, publisherName string, locale uint32) (windows.Handle, error) {
	pub, err := syscall.UTF16PtrFromString(publisherName)
	if err != nil {
		return 0, fmt.Errorf("syscall.UTF16PtrFromString failed: %v", err)
	}

	return wevtapi.EvtOpenPublisherMetadata(session, pub, nil, locale, mustBeZero)
}
