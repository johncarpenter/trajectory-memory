package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootFrom_GitMarker(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	subDir := filepath.Join(projectDir, "src", "pkg")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Should find project root from subdirectory
	root := FindRootFrom(subDir)
	if root != projectDir {
		t.Errorf("expected %s, got %s", projectDir, root)
	}
}

func TestFindRootFrom_ClaudeMDMarker(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	subDir := filepath.Join(projectDir, "deep", "nested")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create CLAUDE.md marker
	if err := os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("# Project"), 0644); err != nil {
		t.Fatal(err)
	}

	root := FindRootFrom(subDir)
	if root != projectDir {
		t.Errorf("expected %s, got %s", projectDir, root)
	}
}

func TestFindRootFrom_NoMarker(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "no", "markers", "here")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Should return the starting directory when no marker found
	root := FindRootFrom(subDir)
	if root != subDir {
		t.Errorf("expected %s, got %s", subDir, root)
	}
}

func TestHashPath(t *testing.T) {
	tests := []struct {
		path     string
		wantLen  int
	}{
		{"/Users/john/project-a", 8},
		{"/Users/john/project-b", 8},
		{".", 8},
	}

	for _, tc := range tests {
		hash := HashPath(tc.path)
		if len(hash) != tc.wantLen {
			t.Errorf("HashPath(%q) = %q, want length %d", tc.path, hash, tc.wantLen)
		}
	}

	// Different paths should produce different hashes
	hashA := HashPath("/project-a")
	hashB := HashPath("/project-b")
	if hashA == hashB {
		t.Errorf("different paths produced same hash: %s", hashA)
	}
}

func TestSocketPath(t *testing.T) {
	path := SocketPath("/Users/john/myproject")

	// Should be in /tmp
	if filepath.Dir(path) != "/tmp" {
		t.Errorf("socket not in /tmp: %s", path)
	}

	// Should have trajectory-memory prefix
	base := filepath.Base(path)
	if len(base) < len("trajectory-memory-") {
		t.Errorf("socket name too short: %s", base)
	}

	// Should end with .sock
	if filepath.Ext(path) != ".sock" {
		t.Errorf("socket should end with .sock: %s", path)
	}
}

func TestDataDir(t *testing.T) {
	dir := DataDir("/Users/john/myproject")
	want := "/Users/john/myproject/.trajectory-memory"
	if dir != want {
		t.Errorf("DataDir() = %s, want %s", dir, want)
	}
}

func TestDBPath(t *testing.T) {
	path := DBPath("/Users/john/myproject")
	want := "/Users/john/myproject/.trajectory-memory/tm.db"
	if path != want {
		t.Errorf("DBPath() = %s, want %s", path, want)
	}
}
