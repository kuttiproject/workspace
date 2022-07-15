package workspace

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kuttiproject/kuttilog"
)

// ChecksumFile calculates an SHA256 checksum of a file.
func ChecksumFile(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	result := fmt.Sprintf("%x", h.Sum(nil))
	return result, nil
}

// ProgressFunc is a callback to indicate io operation progress.
// It is called with two numeric parameters. The first indicates
// current progress. The second can optionally indicate total
// progress, or 0. The meaning of these numbers is up to the
// implementation. They can be completely ignored if the purpose
// is to indicate *some* progress.
type ProgressFunc func(progress int64, total int64)

type progressreader struct {
	io.Reader
	current  int64
	total    int64
	callback ProgressFunc
}

func (pr *progressreader) Read(dst []byte) (int, error) {
	n, err := pr.Reader.Read(dst)

	if err == nil && pr.callback != nil {
		pr.current += int64(n)
		pr.callback(pr.current, pr.total)
	}

	return n, err
}

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

	destination, err := os.Create(destpath)
	if err != nil {
		return err
	}
	defer destination.Close()

	var sourcereader io.Reader
	if progress != nil {
		sourcereader = &progressreader{
			source,
			0,
			sourceFileStat.Size(),
			progress,
		}
	} else {
		sourcereader = source
	}

	kuttilog.Printf(kuttilog.Debug, "Copying %s to %s:\n", sourcepath, destpath)

	buf := make([]byte, buffersize)
	for {
		n, err := sourcereader.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 && err == io.EOF {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}

	return err
}

func downloadfile(url string, filepath string, progress ProgressFunc) error {
	kuttilog.Printf(kuttilog.Debug, "Connecting to %s...", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP client returned the status: %v:%v", resp.StatusCode, resp.Status)
	}

	tmpfilepath := filepath + ".download"
	out, err := os.Create(tmpfilepath)
	if err != nil {
		return err
	}

	var respreader io.Reader
	if progress != nil {
		respreader = &progressreader{
			resp.Body,
			0,
			resp.ContentLength,
			progress,
		}
	} else {
		respreader = resp.Body
	}

	if _, err = io.Copy(out, respreader); err != nil {
		out.Close()
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	kuttilog.Printf(kuttilog.Debug, "Saved to temporary file %v.", tmpfilepath)

	// Check and remove destination path if it exists
	// Windows may cause a problem otherwise
	_, err = os.Stat(filepath)
	if err == nil {
		os.RemoveAll(filepath)
	}

	if err = os.Rename(tmpfilepath, filepath); err != nil {
		return err
	}

	kuttilog.Printf(kuttilog.Debug, "Downloaded to file %v.", filepath)

	return nil
}

// CopyFile copies a file in chunks of the specified size.
func CopyFile(sourcepath string, destpath string, buffersize int64, overwrite bool) error {
	return copyfile(sourcepath, destpath, buffersize, overwrite, nil)
}

// CopyFileWithProgress copies a file in chunks of the specified size,
// and reports progress via the supplied callback. The progress callback
// reports current and  total numbers as bytes.
func CopyFileWithProgress(sourcepath string, destpath string, buffersize int64, overwrite bool, progress ProgressFunc) error {
	return copyfile(sourcepath, destpath, buffersize, overwrite, progress)
}

// DownloadFile downloads a file from a url.
func DownloadFile(url string, filepath string) error {
	return downloadfile(url, filepath, nil)
}

// DownloadFileWithProgress downloads a file from a url and reports progress
// via the supplied callback. The progress callback reports current and total
// numbers as bytes.
func DownloadFileWithProgress(url string, filepath string, progress ProgressFunc) error {
	return downloadfile(url, filepath, progress)
}

// RemoveFile deletes a file.
func RemoveFile(filepath string) error {
	return os.Remove(filepath)
}
