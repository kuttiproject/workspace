package workspace

import (
	"os"
	"time"
)

// Seams for tests.
var (
	readfilefunc          = os.ReadFile
	istransientfileerrorf = istransientfileerror
	readretryattempts     = 100
	readretrydelay        = 10 * time.Millisecond
)

// ReadFileRetrying reads a whole file, like os.ReadFile, but retries for
// about a second if the read fails because the file is momentarily in use.
//
// On Windows, opening a file while another process replaces it can fail
// briefly with a sharing violation. Files written with AtomicWriteFile
// are replaced in this way. That is not a missing or partial file, and
// retrying gets either the old or the new content. Other errors, including
// the file not existing, are returned immediately. On other platforms,
// this behaves like os.ReadFile.
func ReadFileRetrying(path string) ([]byte, error) {
	var data []byte
	var err error

	for attempt := 0; attempt < readretryattempts; attempt++ {
		data, err = readfilefunc(path)
		if err == nil || !istransientfileerrorf(err) {
			return data, err
		}

		time.Sleep(readretrydelay)
	}

	return nil, err
}
