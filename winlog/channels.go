// +build windows

package winlog

import (
	"fmt"
	"syscall"

	"github.com/google/winops/winlog/wevtapi"
)

// AvailableChannels returns a slice of channels registered on the system.
func AvailableChannels() ([]string, error) {
	h, err := wevtapi.EvtOpenChannelEnum(localMachine, mustBeZero)
	if err != nil {
		return nil, fmt.Errorf("wevtapi.EvtOpenChannelEnum failed: %v", err)
	}
	defer Close(h)

	// Enumerate all the channel names. Dynamically allocate the buffer to receive
	// channel names depending on the buffer size required as reported by the API.
	var channels []string
	buf := make([]uint16, 1)
	for {
		var bufferUsed uint32
		err := wevtapi.EvtNextChannelPath(h, uint32(len(buf)), &buf[0], &bufferUsed)
		switch err {
		case nil:
			channels = append(channels, syscall.UTF16ToString(buf[:bufferUsed]))
		case syscall.ERROR_INSUFFICIENT_BUFFER:
			// Grow buffer.
			buf = make([]uint16, bufferUsed)
			continue
		case syscall.Errno(259): // ERROR_NO_MORE_ITEMS
			return channels, nil
		default:
			return nil, fmt.Errorf("wevtapi.EvtNextChannelPath failed: %v", err)
		}
	}
}
