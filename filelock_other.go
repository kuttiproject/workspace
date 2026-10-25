//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package workspace

import "os"

// On platforms that kutti does not support, the lock is a no-op.
func trylockfile(file *os.File) (bool, error) {
	return true, nil
}

func unlockfile(file *os.File) {}
