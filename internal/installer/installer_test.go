package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestInstaller(t *testing.T) (*Installer, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "installer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dataDir := filepath.Join(tmpDir, ".trajectory-memory")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create a project directory with .claude folder
	projectDir := filepath.Join(tmpDir, "project")
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Change to project directory
	oldWd, _ := os.Getwd()
	os.Chdir(projectDir)

	cleanup := func() {
		os.Chdir(oldWd)
		os.RemoveAll(tmpDir)
	}

	return NewInstaller(dataDir), projectDir, cleanup
}

func TestInstallCreatesHookScript(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify hook script exists
	hookPath := installer.GetHookPath()
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook script not found: %v", err)
	}

	// Check it's executable
	if info.Mode()&0111 == 0 {
		t.Error("hook script should be executable")
	}

	// Check content
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook script: %v", err)
	}

	if len(content) == 0 {
		t.Error("hook script should not be empty")
	}
	if string(content[:2]) != "#!" {
		t.Error("hook script should start with shebang")
	}
}

func TestInstallUpdatesSettings(t *testing.T) {
	installer, projectDir, cleanup := setupTestInstaller(t)
	defer cleanup()

	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Read settings
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	if settings.Hooks == nil {
		t.Fatal("hooks should be set")
	}
	if len(settings.Hooks.PostToolUse) != 1 {
		t.Errorf("expected 1 PostToolUse hook, got %d", len(settings.Hooks.PostToolUse))
	}

	hook := settings.Hooks.PostToolUse[0]
	if len(hook.Hooks) != 1 {
		t.Errorf("expected 1 hook command, got %d", len(hook.Hooks))
	}
	if hook.Hooks[0].Command != installer.GetHookPath() {
		t.Errorf("hook path mismatch: got %s, want %s", hook.Hooks[0].Command, installer.GetHookPath())
	}
	if hook.Hooks[0].Type != "command" {
		t.Errorf("hook type should be 'command', got %s", hook.Hooks[0].Type)
	}
}

func TestInstallPreservesExistingHooks(t *testing.T) {
	installer, projectDir, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Create settings with existing hook (new format)
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	existingSettings := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "/path/to/existing-hook.sh"}]
      }
    ]
  }
}`
	os.WriteFile(settingsPath, []byte(existingSettings), 0644)

	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Read settings
	data, _ := os.ReadFile(settingsPath)
	var settings ClaudeSettings
	json.Unmarshal(data, &settings)

	if len(settings.Hooks.PostToolUse) != 2 {
		t.Errorf("expected 2 PostToolUse hooks, got %d", len(settings.Hooks.PostToolUse))
	}

	// Check existing hook is preserved
	found := false
	for _, entry := range settings.Hooks.PostToolUse {
		for _, h := range entry.Hooks {
			if h.Command == "/path/to/existing-hook.sh" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("existing hook should be preserved")
	}
}

func TestInstallPreservesOtherSettings(t *testing.T) {
	installer, projectDir, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Create settings with other fields
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	existingSettings := `{
  "permissions": {
    "allow": ["Bash"]
  },
  "model": "opus"
}`
	os.WriteFile(settingsPath, []byte(existingSettings), 0644)

	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Read settings
	data, _ := os.ReadFile(settingsPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if raw["permissions"] == nil {
		t.Error("permissions should be preserved")
	}
	if raw["model"] != "opus" {
		t.Error("model should be preserved")
	}
}

func TestInstallAlreadyInstalled(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	// First install
	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("First install failed: %v", err)
	}

	// Second install should fail
	err = installer.Install(InstallOptions{})
	if err == nil {
		t.Error("expected error on duplicate install")
	}
}

func TestUninstall(t *testing.T) {
	installer, projectDir, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Install first
	err := installer.Install(InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify installed
	if !installer.IsInstalled(InstallOptions{}) {
		t.Error("should be installed")
	}

	// Uninstall
	err = installer.Uninstall(InstallOptions{})
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Verify hook script removed
	hookPath := installer.GetHookPath()
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook script should be removed")
	}

	// Verify settings updated
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	data, _ := os.ReadFile(settingsPath)
	var settings ClaudeSettings
	json.Unmarshal(data, &settings)

	if settings.Hooks != nil && len(settings.Hooks.PostToolUse) > 0 {
		t.Error("hooks should be empty after uninstall")
	}

	// Verify not installed
	if installer.IsInstalled(InstallOptions{}) {
		t.Error("should not be installed")
	}
}

func TestUninstallPreservesOtherHooks(t *testing.T) {
	installer, projectDir, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Install first
	installer.Install(InstallOptions{})

	// Add another hook manually
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	data, _ := os.ReadFile(settingsPath)
	var settings ClaudeSettings
	json.Unmarshal(data, &settings)

	settings.Hooks.PostToolUse = append(settings.Hooks.PostToolUse, HookEntry{
		Matcher: "Bash",
		Hooks: []Hook{
			{Type: "command", Command: "/path/to/other-hook.sh"},
		},
	})

	updatedData, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, updatedData, 0644)

	// Uninstall
	installer.Uninstall(InstallOptions{})

	// Verify other hook preserved
	data, _ = os.ReadFile(settingsPath)
	json.Unmarshal(data, &settings)

	if len(settings.Hooks.PostToolUse) != 1 {
		t.Errorf("expected 1 hook remaining, got %d", len(settings.Hooks.PostToolUse))
	}
	if len(settings.Hooks.PostToolUse[0].Hooks) == 0 || settings.Hooks.PostToolUse[0].Hooks[0].Command != "/path/to/other-hook.sh" {
		t.Error("other hook should be preserved")
	}
}

func TestUninstallNoSettings(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Uninstall without installing - should not error
	err := installer.Uninstall(InstallOptions{})
	if err != nil {
		t.Errorf("Uninstall should not error when nothing installed: %v", err)
	}
}

func TestIsInstalled(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	// Initially not installed
	if installer.IsInstalled(InstallOptions{}) {
		t.Error("should not be installed initially")
	}

	// Install
	installer.Install(InstallOptions{})

	// Now installed
	if !installer.IsInstalled(InstallOptions{}) {
		t.Error("should be installed after Install()")
	}

	// Uninstall
	installer.Uninstall(InstallOptions{})

	// Not installed again
	if installer.IsInstalled(InstallOptions{}) {
		t.Error("should not be installed after Uninstall()")
	}
}

func TestGetMCPConfig(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	config := installer.GetMCPConfig()

	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(config), &parsed)
	if err != nil {
		t.Fatalf("MCP config should be valid JSON: %v", err)
	}

	servers, ok := parsed["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("should have mcpServers field")
	}

	tm, ok := servers["trajectory-memory"].(map[string]interface{})
	if !ok {
		t.Fatal("should have trajectory-memory server")
	}

	if tm["command"] != "trajectory-memory" {
		t.Error("command should be 'trajectory-memory'")
	}
}

func TestGetClaudeMDSnippet(t *testing.T) {
	installer, _, cleanup := setupTestInstaller(t)
	defer cleanup()

	snippet := installer.GetClaudeMDSnippet()

	if len(snippet) == 0 {
		t.Error("CLAUDE.md snippet should not be empty")
	}

	// Check for key content
	expectedPhrases := []string{
		"trajectory_search",
		"trajectory_summarize",
		"start logging",
		"stop logging",
	}

	for _, phrase := range expectedPhrases {
		if !containsString(snippet, phrase) {
			t.Errorf("snippet should contain '%s'", phrase)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s[1:], substr) || s[:len(substr)] == substr)
}
