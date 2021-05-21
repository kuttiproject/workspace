package workspace

import (
	"errors"
	"io/ioutil"
	"os"
	"path"

	"github.com/kuttiproject/kuttilog"
)

type fileConfigManager struct {
	configfilename string
	configdata     Configdata
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
		cm.configdata.Setdefaults()
		return cm.Save()
	}

	if err != nil {
		return err
	}

	err = cm.configdata.Deserialize(data)
	if err != nil {
		kuttilog.Printf(
			kuttilog.Verbose,
			"Error reading config file '%s':%v. Loading defaults.",
			cm.configfilename,
			err,
		)
		cm.configdata.Setdefaults()
		cm.Save()
		return err
	}

	kuttilog.Printf(
		kuttilog.Verbose,
		"Config file '%s' loaded. Data is: %s",
		cm.configfilename,
		data,
	)

	return nil
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
	cm.configdata.Setdefaults()
}

func getconfigfilepath(configFileName string) (string, error) {
	configPath, err := Configdir()
	if err != nil {
		return "", err
	}

	datafilepath := path.Join(configPath, configFileName)
	return datafilepath, nil
}

// saveconfigfile saves the specified data into the named file in the kutti config directory.
func saveconfigfile(configfilename string, data []byte) error {
	datafilepath, err := getconfigfilepath(configfilename)
	if err != nil {
		return err
	}

	file, err := os.Create(datafilepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// loadconfigfile loads data from the named file in the kutti config directory.
// If the named file does not exist, the second returned value is true
func loadconfigfile(configfilename string) ([]byte, bool, error) {
	datafilepath, err := getconfigfilepath(configfilename)
	if err != nil {
		return nil, false, err
	}
	_, err = os.Stat(datafilepath)
	if os.IsNotExist(err) {
		return nil, true, err
	}

	if err != nil {
		return nil, false, err
	}

	data, err := ioutil.ReadFile(datafilepath)

	if err != nil {
		return nil, false, err
	}

	return data, false, nil
}

// NewFileConfigmanager returns a Configmanager that manages data in a file saved
// under the current workspace's configuration directory.
func NewFileConfigmanager(filename string, s Configdata) (Configmanager, error) {
	if filename == "" || s == nil {
		return nil,
			errors.New("must provide configuration file name and serializer")
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
