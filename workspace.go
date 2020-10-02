package workspace

import (
	"errors"
	"os"
	"path"
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

// Configdir returns the full path where config files reside.
// If the directory does not exist, it is created.
func Configdir() (string, error) {
	if workspace == "" {
		return defaultconfigdir()
	}

	return ensuresubdirectory(workspace, "kutti-config")
}

// Configsubdir returns the full path to a subdirectory under the Configdir.
// If the directory does not exist, it is created.
func Configsubdir(subpath string) (string, error) {
	configdir, err := Configdir()
	if err != nil {
		return "", err
	}

	return ensuresubdirectory(configdir, subpath)
}

// Cachedir returns the location where cached files should reside.
// If the directory does not exist, it is created.
func Cachedir() (string, error) {
	if workspace == "" {
		return defaultcachedir()
	}

	return ensuresubdirectory(workspace, "kutti-cache")
}

// Cachesubdir returns the full path to a subdirectory under the Cachedir.
// If the directory does not exist, it is created.
func Cachesubdir(subpath string) (string, error) {
	cachedir, err := Cachedir()
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
	result := path.Join(directorypath, subpath)

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
