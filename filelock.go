package workspace

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// lockPollInterval is how often LockFile retries while it waits.
var lockPollInterval = 100 * time.Millisecond

// LockFile takes an exclusive lock on a file, creating the file if it does
// not exist, and returns a function that releases the lock. It is meant to
// serialize operations between processes, and between goroutines, that use
// the same lock file. The lock file is only ever locked. Its content is not
// used, and it is not removed.
//
// If another holder has the lock, LockFile waits. It calls onwait, if it is
// not nil, once, when it first has to wait. If timeout is greater than zero,
// LockFile gives up with an error after that long. A timeout of zero or less
// waits indefinitely.
//
// The lock is released by the returned function, which is safe to call more
// than once, or by the operating system if the process ends. The lock is
// advisory: it only affects code that also calls LockFile.
func LockFile(path string, timeout time.Duration, onwait func()) (func(), error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("could not open lock file '%s': %w", path, err)
	}

	start := time.Now()
	announced := false
	for {
		locked, err := trylockfile(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("could not lock '%s': %w", path, err)
		}

		if locked {
			break
		}

		if timeout > 0 && time.Since(start) > timeout {
			file.Close()
			return nil, fmt.Errorf(
				"timed out after %v waiting for the lock on '%s'",
				timeout,
				path,
			)
		}

		if !announced {
			announced = true
			if onwait != nil {
				onwait()
			}
		}

		time.Sleep(lockPollInterval)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			unlockfile(file)
			file.Close()
		})
	}, nil
}
