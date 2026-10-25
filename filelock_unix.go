//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package workspace

import (
	"errors"
	"os"
	"syscall"
)

// trylockfile tries to take an exclusive lock without waiting.
// It returns false, with no error, if another holder has it.
func trylockfile(file *os.File) (bool, error) {
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK):
			return false, nil
		default:
			return false, err
		}
	}
}

func unlockfile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
