package workspace

// Configdata provides methods for serializing and deserializing
// config data, and setting default values. These methods will be called
// by a ConfigManager as appropriate.
// Users should create a config data structure of their choice, implement
// this interface, and use an instance of the data structure to initialize
// a ComfigManager.
type Configdata interface {
	Serialize() ([]byte, error)
	Deserialize([]byte) error
	Setdefaults()
}
