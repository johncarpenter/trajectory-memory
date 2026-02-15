// Package config handles application configuration.
package config

import (
	"os"

	"github.com/johncarpenter/trajectory-memory/internal/project"
)

// Config holds the application configuration.
type Config struct {
	ProjectRoot string
	DBPath      string
	SocketPath  string
	DataDir     string
}

// Load creates a Config from environment variables with defaults.
// Uses per-project paths based on detected project root.
func Load() *Config {
	// Detect project root first
	projectRoot := project.FindRoot()

	cfg := &Config{
		ProjectRoot: projectRoot,
		DataDir:     project.DataDir(projectRoot),
		DBPath:      project.DBPath(projectRoot),
		SocketPath:  project.SocketPath(projectRoot),
	}

	// Environment variables override defaults
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
