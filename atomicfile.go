package workspace

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// AtomicWriteFile writes data to a file so that readers see either the
// old content or the new content, never a partial file. It writes a
// temporary file in the same directory, syncs it, and renames it over the
// destination. If anything fails, the destination is unchanged and the
// temporary file is removed. The file gets the permissions that os.Create
// would give it.
//
// Use ReadFileRetrying to read such a file where other processes may be
// replacing it, because on Windows a read can fail briefly during the
// replacement.
func AtomicWriteFile(path string, data []byte) error {
	return atomicwritefile(path, data)
}

// atomicwritefile writes data to a file so that readers see either the
// old content or the new content, never a partial file. It writes a
// temporary file in the same directory, syncs it, and renames it over the
// destination.
func atomicwritefile(path string, data []byte) error {
	tmppath := uniquetempname(path)

	file, err := os.OpenFile(tmppath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeerr := file.Close(); err == nil {
		err = closeerr
	}
	if err != nil {
		os.Remove(tmppath)
		return err
	}

	err = renamereplace(tmppath, path)
	if err != nil {
		os.Remove(tmppath)
		return err
	}

	return nil
}

// tempnamenow and tempnamecounter make temporary file names unique.
// A clock alone is not enough: on Windows it ticks every 100 nanoseconds,
// and two goroutines can get the same reading.
var (
	tempnamenow     = func() int64 { return time.Now().UnixNano() }
	tempnamecounter atomic.Uint64
)

// uniquetempname returns a name for a temporary file next to path, that
// is unique to this process, goroutine and moment.
func uniquetempname(path string) string {
	return fmt.Sprintf(
		"%s.tmp-%d-%d-%d",
		path,
		os.Getpid(),
		tempnamenow(),
		tempnamecounter.Add(1),
	)
}

// renamereplace renames a file, replacing the destination if it exists.
// On Windows, the replacement can fail briefly if another process has the
// destination open, so it is retried.
func renamereplace(from string, to string) error {
	attempts := 1
	if runtime.GOOS == "windows" {
		attempts = 50
	}

	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		err = os.Rename(from, to)
		if err == nil {
			return nil
		}

		if attempt < attempts-1 {
			time.Sleep(20 * time.Millisecond)
		}
	}

	return err
}
