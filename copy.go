package workspace

import (
	"fmt"
	"io"
	"os"

	"github.com/kuttiproject/kuttilog"
)

// minimumcopychunk is the smallest amount copied between progress reports.
// The buffersize that callers pass is a minimum too: copying in large
// chunks between open files lets the operating system do the copying
// where it can (for example, copy_file_range on Linux).
const minimumcopychunk = 4 << 20

// copyfile copies a regular file. The data goes to a temporary file next
// to the destination, which is synced and then replaces the destination
// atomically. A failed copy never changes the destination, and the
// temporary file is removed. Copying a file onto itself is safe.
//
// If overwrite is false, copyfile fails if the destination exists. This
// is a check made before copying starts, so a file created by someone else
// in the meantime can still be replaced.
//
// The buffersize is the least number of bytes to copy between progress
// reports. Values below 4 MiB, including zero, are raised to 4 MiB.
func copyfile(sourcepath string, destpath string, buffersize int64, overwrite bool, progress ProgressFunc) error {
	sourceFileStat, err := os.Stat(sourcepath)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", sourcepath)
	}

	if !overwrite {
		_, err = os.Stat(destpath)
		if err == nil {
			return fmt.Errorf("destination path %s already exists", destpath)
		}
	}

	source, err := os.Open(sourcepath)
	if err != nil {
		return err
	}
	defer source.Close()

	kuttilog.Printf(kuttilog.Debug, "Copying %s to %s:\n", sourcepath, destpath)

	tmppath, err := copytotempfile(source, sourceFileStat.Size(), destpath, buffersize, progress)

	// Close the source before replacing anything. On Windows, a file that
	// is open cannot be replaced, and the source can be the destination.
	source.Close()

	if err != nil {
		return err
	}

	err = renamereplace(tmppath, destpath)
	if err != nil {
		os.Remove(tmppath)
		return err
	}

	return nil
}

// copytotempfile copies exactly size bytes from source into a new
// temporary file next to destpath, syncs it, and returns its path.
// If anything fails, including the source holding a different number of
// bytes than expected, the temporary file is removed.
func copytotempfile(source io.Reader, size int64, destpath string, buffersize int64, progress ProgressFunc) (string, error) {
	chunk := buffersize
	if chunk < minimumcopychunk {
		chunk = minimumcopychunk
	}

	// Same permissions as os.Create. os.CreateTemp would make 0600.
	tmppath := uniquetempname(destpath)
	out, err := os.OpenFile(tmppath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		out.Close()
		os.Remove(tmppath)
		return "", err
	}

	var copied int64
	for {
		n, err := io.CopyN(out, source, chunk)
		copied += n
		if n > 0 && progress != nil {
			progress(copied, size)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fail(err)
		}
	}

	if copied != size {
		return fail(fmt.Errorf(
			"source changed during copy: copied %d bytes, expected %d",
			copied,
			size,
		))
	}

	// Data must be on disk before it replaces anything.
	err = out.Sync()
	if err != nil {
		return fail(err)
	}

	err = out.Close()
	if err != nil {
		os.Remove(tmppath)
		return "", err
	}

	return tmppath, nil
}
