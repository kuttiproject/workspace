package workspace

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	workspace = ""
)

// Set sets the workspace to the path specified.
// Config and Cache directories will be set as subdirectories under the specified path,
// called kutti-config and kutti-cache respectively.
func Set(workspacepath string) error {
	err := ensuredirectory(workspacepath)
	if err != nil {
		return err
	}

	workspace = workspacepath
	return nil
}

// Reset resets the workspace to the default location.
// Config and Cache directories will be set as subdirectories
// called kutti under the current user's config and cache locations
// respectively.
func Reset() {
	workspace = ""
}

// ConfigDir returns the full path where config files reside.
// If the directory does not exist, it is created.
func ConfigDir() (string, error) {
	if workspace == "" {
		return defaultconfigdir()
	}

	return ensuresubdirectory(workspace, "kutti-config")
}

// CacheDir returns the location where cached files should reside.
// If the directory does not exist, it is created.
func CacheDir() (string, error) {
	if workspace == "" {
		return defaultcachedir()
	}

	return ensuresubdirectory(workspace, "kutti-cache")
}

// CacheSubDir returns the full path to a subdirectory under the CacheDir.
// If the directory does not exist, it is created.
func CacheSubDir(subpath string) (string, error) {
	cachedir, err := CacheDir()
	if err != nil {
		return "", err
	}

	return ensuresubdirectory(cachedir, subpath)
}

func defaultconfigdir() (string, error) {
	result, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return ensuresubdirectory(result, "kutti")
}

func defaultcachedir() (string, error) {
	result, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return ensuresubdirectory(result, "kutti")
}

func ensuresubdirectory(directorypath string, subpath string) (string, error) {
	result := filepath.Join(directorypath, subpath)

	err := ensuredirectory(result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func ensuredirectory(path string) error {
	dirinfo, err := os.Stat(path)
	if err == nil && !dirinfo.IsDir() {
		err = errors.New("not a directory")
	}
	if os.IsNotExist(err) {
		err = os.Mkdir(path, 0755)
	}
	return err
}
