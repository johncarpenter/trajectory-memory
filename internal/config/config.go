// Package config handles application configuration.
package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	DBPath     string
	SocketPath string
	DataDir    string
}

// DefaultDataDir returns the default data directory (~/.trajectory-memory).
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".trajectory-memory"
	}
	return filepath.Join(home, ".trajectory-memory")
}

// DefaultDBPath returns the default database path.
func DefaultDBPath() string {
	return filepath.Join(DefaultDataDir(), "tm.db")
}

// DefaultSocketPath returns the default Unix socket path.
func DefaultSocketPath() string {
	return "/tmp/trajectory-memory.sock"
}

// Load creates a Config from environment variables with defaults.
func Load() *Config {
	cfg := &Config{
		DBPath:     DefaultDBPath(),
		SocketPath: DefaultSocketPath(),
		DataDir:    DefaultDataDir(),
	}

	if path := os.Getenv("TM_DB_PATH"); path != "" {
		cfg.DBPath = path
	}

	if path := os.Getenv("TM_SOCKET_PATH"); path != "" {
		cfg.SocketPath = path
	}

	if path := os.Getenv("TM_DATA_DIR"); path != "" {
		cfg.DataDir = path
	}

	return cfg
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}
