// +build !windows

package simple

// NewWindowsEvent returns nil for linux and darwin platform.
func NewWindowsEvent() Event {
	return nil
}
