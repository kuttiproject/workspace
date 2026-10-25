//go:build windows

package workspace

import (
	"errors"
	"syscall"
)

const (
	errorAccessDenied     = syscall.Errno(5)
	errorSharingViolation = syscall.Errno(32)
	errorLockViolation    = syscall.Errno(33)
)

// istransientfileerror reports whether err is the kind of failure that
// Windows gives for a file that is momentarily in use by another process.
func istransientfileerror(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}

	switch errno {
	case errorAccessDenied, errorSharingViolation, errorLockViolation:
		return true
	}

	return false
}
