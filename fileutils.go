package workspace

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
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
