package sarin

import "golang.org/x/sys/unix"

// isForeground reports false only for a background job, which raw mode would stop with SIGTTOU.
func isForeground(fd uintptr) bool {
	// pid_t is 32 bits, a 32 bit read keeps big-endian right.
	pgrp, err := unix.IoctlGetUint32(int(fd), unix.TIOCGPGRP)
	return err != nil || int(pgrp) == unix.Getpgrp()
}
