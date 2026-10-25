package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kuttiproject/kuttilog"
)

type fileConfigManager struct {
	configfilename string
	configdata     ConfigData
}

// Load loads a saved config, or initializes default values
func (cm *fileConfigManager) Load() error {
	data, notexist, err := loadconfigfile(cm.configfilename)
	if notexist {
		kuttilog.Printf(
			kuttilog.Verbose,
			"Config file '%s' does not exist. Loading defaults.",
			cm.configfilename,
		)
		cm.configdata.SetDefaults()
		return cm.Save()
	}

	if err != nil {
		return err
	}

	err = cm.configdata.Deserialize(data)
	if err != nil {
		return cm.recoverfromcorruptfile(err)
	}

	kuttilog.Printf(
		kuttilog.Verbose,
		"Config file '%s' loaded. Data is: %s",
		cm.configfilename,
		data,
	)

	return nil
}

// recoverfromcorruptfile is called when a config file exists, but cannot
// be deserialized. It sets the file aside, so that nothing is lost, loads
// default values, and saves them. A config file is not changed in any
// way until it has been set aside.
//
// If it succeeds, Load succeeds: the manager is in a usable state, with
// default values.
func (cm *fileConfigManager) recoverfromcorruptfile(parseerr error) error {
	backuppath, err := setasideconfigfile(cm.configfilename)
	if err != nil {
		return fmt.Errorf(
			"config file '%s' could not be read (%v), and could not be set aside: %w",
			cm.configfilename,
			parseerr,
			err,
		)
	}

	kuttilog.Printf(
		kuttilog.Quiet,
		"Config file '%s' could not be read: %v. It has been moved to '%s', and default values are in use.",
		cm.configfilename,
		parseerr,
		backuppath,
	)

	cm.configdata.SetDefaults()
	return cm.Save()
}

// Save saves a config
func (cm *fileConfigManager) Save() error {
	kuttilog.Printf(
		kuttilog.Verbose,
		"Saving config file '%s'...",
		cm.configfilename,
	)
	data, err := cm.configdata.Serialize()
	if err != nil {
		kuttilog.Printf(
			kuttilog.Debug,
			"Error Saving config file '%s': %v.",
			cm.configfilename,
			err,
		)
		return err
	}

	return saveconfigfile(cm.configfilename, data)
}

// Reset resets a config to default values
func (cm *fileConfigManager) Reset() {
	cm.configdata.SetDefaults()
}

func getconfigfilepath(configFileName string) (string, error) {
	configPath, err := ConfigDir()
	if err != nil {
		return "", err
	}

	datafilepath := filepath.Join(configPath, configFileName)
	return datafilepath, nil
}

// saveconfigfile saves the specified data into the named file in the kutti
// config directory. The file is replaced atomically.
func saveconfigfile(configfilename string, data []byte) error {
	datafilepath, err := getconfigfilepath(configfilename)
	if err != nil {
		return err
	}

	return atomicwritefile(datafilepath, data)
}

// loadconfigfile loads data from the named file in the kutti config directory.
// If the named file does not exist, the second returned value is true
func loadconfigfile(configfilename string) ([]byte, bool, error) {
	datafilepath, err := getconfigfilepath(configfilename)
	if err != nil {
		return nil, false, err
	}

	data, err := ReadFileRetrying(datafilepath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, true, err
	}

	if err != nil {
		return nil, false, err
	}

	return data, false, nil
}

// setasideconfigfile renames the named file in the kutti config directory
// to <name>.<timestamp>.corrupt, in the same directory, and returns the
// new path. The timestamp is in UTC, like 20060102T150405.000Z.
//
// If the file does not exist any more, for example because another process
// set it aside first, that is not an error.
func setasideconfigfile(configfilename string) (string, error) {
	datafilepath, err := getconfigfilepath(configfilename)
	if err != nil {
		return "", err
	}

	backuppath := fmt.Sprintf(
		"%s.%s.corrupt",
		datafilepath,
		time.Now().UTC().Format("20060102T150405.000Z"),
	)

	err = os.Rename(datafilepath, backuppath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	return backuppath, nil
}

// NewFileConfigManager returns a ConfigManager that manages data in a file saved
// under the current workspace's configuration directory.
// The filename parameter needs a filename without a path.
func NewFileConfigManager(filename string, s ConfigData) (ConfigManager, error) {
	if filename == "" || s == nil {
		return nil,
			errors.New("must provide configuration file name and serializer")
	}

	dirname, filename := filepath.Split(filename)
	if dirname != "" {
		return nil,
			errors.New("configuration file name must not have a path")
	}

	result := &fileConfigManager{
		configfilename: filename,
		configdata:     s,
	}
	err := result.Load()
	if err != nil {
		return nil, err
	}

	return result, nil
}
