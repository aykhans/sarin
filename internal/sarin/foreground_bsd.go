//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package sarin

import "golang.org/x/sys/unix"

// isForeground is the same check as on linux, see foreground_linux.go.
func isForeground(fd uintptr) bool {
	v, err := unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
	// The kernel writes a 32 bit pid_t, which lands in the high half of a 64 bit big-endian int.
	pgrp := int(uint32(int64(v)) | uint32(int64(v)>>32))
	return err != nil || pgrp == unix.Getpgrp()
}
