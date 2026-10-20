//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package sarin

// isForeground has no background job check on this platform.
func isForeground(uintptr) bool {
	return true
}
