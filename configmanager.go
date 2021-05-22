package workspace

// Configmanager provides methods to save and load configuration data.
// The actual location to save to and load from varies per implementation.
type Configmanager interface {
	Load() error // Load loads a saved config, or initializes default values
	Save() error // Save saves a config
	Reset()      // Reset resets a config to default values
}
