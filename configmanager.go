package workspace

// ConfigManager provides methods to save and load configuration data.
// The actual location to save to and load from varies per implementation.
type ConfigManager interface {
	// Load loads a saved config, or initializes default values
	Load() error
	// Save saves a config
	Save() error
	// Reset resets a config to default values
	Reset()
}
