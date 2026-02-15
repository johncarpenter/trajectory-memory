// Package project provides project root detection and path utilities.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// FindRoot detects the project root by walking up the directory tree.
// It looks for markers in this order:
//  1. .git directory
//  2. CLAUDE.md file
//  3. .claude directory
//
// If no marker is found, returns the current working directory.
func FindRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return FindRootFrom(wd)
}

// FindRootFrom detects the project root starting from the given directory.
func FindRootFrom(startDir string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return startDir
	}

	for {
		// Check for project markers
		if isProjectRoot(dir) {
			return dir
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, use original directory
			break
		}
		dir = parent
	}

	// No marker found, use starting directory
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return startDir
	}
	return abs
}

// isProjectRoot checks if a directory contains project markers.
func isProjectRoot(dir string) bool {
	markers := []string{
		filepath.Join(dir, ".git"),
		filepath.Join(dir, "CLAUDE.md"),
		filepath.Join(dir, ".claude"),
	}

	for _, marker := range markers {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

// HashPath generates a short hash from a path for use in socket names.
// Returns the first 8 characters of the SHA256 hash.
func HashPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	hash := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(hash[:])[:8]
}

// SocketPath returns the socket path for a given project root.
// Format: /tmp/trajectory-memory-{hash}.sock
func SocketPath(projectRoot string) string {
	hash := HashPath(projectRoot)
	return filepath.Join("/tmp", "trajectory-memory-"+hash+".sock")
}

// DataDir returns the data directory for a given project root.
// Format: {projectRoot}/.trajectory-memory
func DataDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".trajectory-memory")
}

// DBPath returns the database path for a given project root.
// Format: {projectRoot}/.trajectory-memory/tm.db
func DBPath(projectRoot string) string {
	return filepath.Join(DataDir(projectRoot), "tm.db")
}
