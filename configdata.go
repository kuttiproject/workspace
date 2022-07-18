package workspace

// ConfigData provides methods for serializing and deserializing
// config data, and setting default values. These methods will be called
// by a ConfigManager as appropriate.
//
// Users should create a config data structure of their choice, implement
// this interface, and use an instance of the data structure to initialize
// a ConfigManager.
type ConfigData interface {
	// Serialize converts the config data structure to a []byte.
	Serialize() ([]byte, error)
	// Deserialize sets the values of the config data structure
	// by reading from a []byte.
	Deserialize([]byte) error
	// SetDefaults sets the config data structure's members to their
	// default values.
	SetDefaults()
}
