// Package installer handles installation and uninstallation of trajectory-memory hooks.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HookScriptContent is the embedded hook script.
// It detects project root and computes socket path to match the MCP server.
const HookScriptContent = `#!/bin/bash
# trajectory-memory hook script
# Reads Claude Code hook payload from stdin and forwards to trajectory-memory ingestion socket
# This script is fire-and-forget - errors are silently ignored to not block CC

# Find project root by looking for markers (same logic as Go code)
find_project_root() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
        if [[ -d "$dir/.git" || -f "$dir/CLAUDE.md" || -d "$dir/.claude" ]]; then
            echo "$dir"
            return
        fi
        dir="$(dirname "$dir")"
    done
    # No marker found, use current directory
    echo "$PWD"
}

# Compute socket path from project root
get_socket_path() {
    local project_root="$1"
    # SHA256 hash, first 8 chars (matches Go implementation)
    local hash=$(echo -n "$project_root" | shasum -a 256 | cut -c1-8)
    echo "/tmp/trajectory-memory-${hash}.sock"
}

# Allow override via environment variable
if [[ -n "$TM_SOCKET_PATH" ]]; then
    SOCKET_PATH="$TM_SOCKET_PATH"
else
    PROJECT_ROOT=$(find_project_root)
    SOCKET_PATH=$(get_socket_path "$PROJECT_ROOT")
fi

PAYLOAD=$(cat)

# Only proceed if socket exists
if [ -S "$SOCKET_PATH" ]; then
    curl -s -X POST --unix-socket "$SOCKET_PATH" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" \
        --max-time 1 \
        http://localhost/step > /dev/null 2>&1 || true
fi
`

// ClaudeSettings represents the Claude Code settings.json structure.
type ClaudeSettings struct {
	Hooks      *HooksConfig           `json:"hooks,omitempty"`
	MCPServers map[string]interface{} `json:"mcpServers,omitempty"`
	Other      map[string]interface{} `json:"-"` // Preserve other fields
}

// HooksConfig contains hook configurations.
type HooksConfig struct {
	PostToolUse []HookEntry `json:"PostToolUse,omitempty"`
}

// HookEntry represents a single hook configuration.
type HookEntry struct {
	Matcher HookMatcher `json:"matcher"`
	Hooks   []Hook      `json:"hooks"`
}

// HookMatcher is a string that matches tool names (use "*" for all tools).
type HookMatcher string

// Hook defines the command to run.
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// InstallOptions configures the installation.
type InstallOptions struct {
	Global bool // Install to user-level settings vs project-level
}

// Installer manages hook installation.
type Installer struct {
	dataDir string
}

// NewInstaller creates a new installer.
func NewInstaller(dataDir string) *Installer {
	return &Installer{dataDir: dataDir}
}

// Install installs the trajectory-memory hooks.
func (i *Installer) Install(opts InstallOptions) error {
	// Create hooks directory
	hooksDir := filepath.Join(i.dataDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Write hook script
	hookPath := filepath.Join(hooksDir, "trajectory-hook.sh")
	if err := os.WriteFile(hookPath, []byte(HookScriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	// Find and update settings
	settingsPath, err := i.findSettingsPath(opts.Global)
	if err != nil {
		return err
	}

	// Read existing settings
	settings, err := i.readSettings(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read settings: %w", err)
	}
	if settings == nil {
		settings = &ClaudeSettings{}
	}

	// Check if already installed
	if i.isInstalled(settings, hookPath) {
		return fmt.Errorf("trajectory-memory is already installed")
	}

	// Add hook entry
	if settings.Hooks == nil {
		settings.Hooks = &HooksConfig{}
	}
	settings.Hooks.PostToolUse = append(settings.Hooks.PostToolUse, HookEntry{
		Matcher: "*", // Match all tools
		Hooks: []Hook{
			{
				Type:    "command",
				Command: hookPath,
			},
		},
	})

	// Write updated settings
	if err := i.writeSettings(settingsPath, settings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// Uninstall removes the trajectory-memory hooks.
func (i *Installer) Uninstall(opts InstallOptions) error {
	hookPath := filepath.Join(i.dataDir, "hooks", "trajectory-hook.sh")

	// Find settings
	settingsPath, err := i.findSettingsPath(opts.Global)
	if err != nil {
		return err
	}

	// Read settings
	settings, err := i.readSettings(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No settings file - just remove hook script
			os.Remove(hookPath)
			return nil
		}
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Remove hook entry
	if settings.Hooks != nil {
		var newHooks []HookEntry
		for _, h := range settings.Hooks.PostToolUse {
			if !i.hookEntryContains(h, hookPath) {
				newHooks = append(newHooks, h)
			}
		}
		settings.Hooks.PostToolUse = newHooks
	}

	// Write updated settings
	if err := i.writeSettings(settingsPath, settings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	// Remove hook script
	os.Remove(hookPath)

	return nil
}

// IsInstalled checks if trajectory-memory is already installed.
func (i *Installer) IsInstalled(opts InstallOptions) bool {
	hookPath := filepath.Join(i.dataDir, "hooks", "trajectory-hook.sh")

	settingsPath, err := i.findSettingsPath(opts.Global)
	if err != nil {
		return false
	}

	settings, err := i.readSettings(settingsPath)
	if err != nil {
		return false
	}

	return i.isInstalled(settings, hookPath)
}

// GetHookPath returns the path where the hook script will be installed.
func (i *Installer) GetHookPath() string {
	return filepath.Join(i.dataDir, "hooks", "trajectory-hook.sh")
}

// GetMCPConfig returns the MCP server configuration snippet.
func (i *Installer) GetMCPConfig() string {
	return `{
  "mcpServers": {
    "trajectory-memory": {
      "command": "trajectory-memory",
      "args": ["serve"]
    }
  }
}`
}

// GetClaudeMDSnippet returns the CLAUDE.md integration snippet.
func (i *Installer) GetClaudeMDSnippet() string {
	return `# Trajectory Memory Integration

This project uses trajectory-memory to record and learn from execution traces.

## Recording Sessions

- To start recording: "start logging" or "record this session"
- To stop recording: "stop logging" or "end recording"
- After stopping, generate a summary and call trajectory_summarize

## Best Practices

- Search for relevant past trajectories when starting complex tasks: use trajectory_search
- Reference high-scoring approaches from similar past sessions
- Score sessions honestly to build useful training data
`
}

func (i *Installer) findSettingsPath(global bool) (string, error) {
	if global {
		// User-level settings: ~/.claude/settings.json
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		settingsDir := filepath.Join(home, ".claude")
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create .claude directory: %w", err)
		}
		return filepath.Join(settingsDir, "settings.json"), nil
	}

	// Project-level settings: .claude/settings.json in current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	settingsDir := filepath.Join(cwd, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .claude directory: %w", err)
	}
	return filepath.Join(settingsDir, "settings.json"), nil
}

func (i *Installer) isInstalled(settings *ClaudeSettings, hookPath string) bool {
	if settings.Hooks == nil {
		return false
	}
	for _, h := range settings.Hooks.PostToolUse {
		if i.hookEntryContains(h, hookPath) {
			return true
		}
	}
	return false
}

func (i *Installer) hookEntryContains(entry HookEntry, hookPath string) bool {
	for _, h := range entry.Hooks {
		if h.Command == hookPath {
			return true
		}
	}
	return false
}

func (i *Installer) readSettings(path string) (*ClaudeSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First unmarshal to get all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Then unmarshal to our struct
	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	// Store other fields
	settings.Other = make(map[string]interface{})
	for k, v := range raw {
		if k != "hooks" && k != "mcpServers" {
			settings.Other[k] = v
		}
	}

	return &settings, nil
}

func (i *Installer) writeSettings(path string, settings *ClaudeSettings) error {
	// Build a map to preserve all fields
	data := make(map[string]interface{})

	// Copy other fields first
	for k, v := range settings.Other {
		data[k] = v
	}

	// Add hooks if present
	if settings.Hooks != nil {
		data["hooks"] = settings.Hooks
	}

	// Add mcpServers if present
	if settings.MCPServers != nil {
		data["mcpServers"] = settings.MCPServers
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(jsonData, '\n'), 0644)
}
