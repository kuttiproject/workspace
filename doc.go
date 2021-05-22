// Package workspace provides config management, cache management and utilities for managing instances of the kutti tool.
//
// Config
//
// A "workspace" has a config directory, where configuration files of all sorts
// can be stored. By default, this is a subdirectory called "kutti" under the
// OS-specific user configuration directory, as returned by os.UserConfigDir().
// This directory is meant to be flat, that is, all configuration files in the
// workspace are to be stored in this directory, and not any subdirectory.
//
// Two interfaces, called Configdata and Configmanager, are provided for managing
// configuration files. Configdata should be implemented for storing and retrieving
// any kind of configuration information. Configmanager manages loading and saving
// this data to some kind of persistent storage. A default implementation of
// ConfigManager is provided by the NewFileConfigmanager() method, which uses
// files in the config directory as persistent storage.
//
// Cache
//
// A workspace has a cache directory, where any data files can be stored. By default,
// this is a subdirectory called "kutti" under the OS-specific user cache directory,
// as returned by os.UserConfigDir().
//
// Data files can be stored directly in a workspace's cache directory, or preferably
// in subdirectories under the cache directory.
//
// Utilities
//
// The workspace package provides utilities for copying files, calculating checksums
// of files, downloading files via HTTP get and running OS processes.
package workspace
