//go:build windows

package workspace

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// trylockfile tries to take an exclusive lock without waiting.
// It returns false, with no error, if another holder has it.
func trylockfile(file *os.File) (bool, error) {
	var overlapped syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		file.Fd(),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 != 0 {
		return true, nil
	}

	if errors.Is(err, errorLockViolation) {
		return false, nil
	}

	return false, err
}

func unlockfile(file *os.File) {
	var overlapped syscall.Overlapped
	procUnlockFileEx.Call(
		file.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
}
