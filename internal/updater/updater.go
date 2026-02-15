// Package updater provides self-update functionality.
package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repoOwner = "johncarpenter"
	repoName  = "trajectory-memory"
	githubAPI = "https://api.github.com"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Updater handles self-updating the binary.
type Updater struct {
	currentVersion string
}

// NewUpdater creates a new Updater instance.
func NewUpdater(currentVersion string) *Updater {
	return &Updater{currentVersion: currentVersion}
}

// CheckForUpdate checks if a new version is available.
func (u *Updater) CheckForUpdate() (*Release, bool, error) {
	release, err := u.getLatestRelease()
	if err != nil {
		return nil, false, err
	}

	// Compare versions (strip 'v' prefix if present)
	current := strings.TrimPrefix(u.currentVersion, "v")
	latest := strings.TrimPrefix(release.TagName, "v")

	// Simple string comparison - works for semver
	hasUpdate := latest != current && current != "dev"

	return release, hasUpdate, nil
}

// Update downloads and installs the latest version.
func (u *Updater) Update(release *Release) error {
	// Find the right asset for this OS/arch
	assetName := u.getAssetName()
	var downloadURL string

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.DownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (looking for %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Download the new binary
	fmt.Printf("Downloading %s...\n", assetName)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp(filepath.Dir(execPath), "trajectory-memory-update-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Handle tarball extraction
	if strings.HasSuffix(assetName, ".tar.gz") {
		if err := u.extractTarGz(resp.Body, tmpFile); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to extract tarball: %w", err)
		}
	} else {
		// Direct binary download
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Backup current binary
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	success = true
	return nil
}

func (u *Updater) getLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPI, repoOwner, repoName)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func (u *Updater) getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map architecture names to match goreleaser conventions
	archName := arch
	switch arch {
	case "amd64":
		archName = "x86_64"
	case "386":
		archName = "i386"
	}

	// Map OS names
	osName := os
	switch os {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	case "windows":
		osName = "Windows"
	}

	return fmt.Sprintf("trajectory-memory_%s_%s.tar.gz", osName, archName)
}

func (u *Updater) extractTarGz(r io.Reader, w io.Writer) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for the binary file
		if header.Typeflag == tar.TypeReg && (header.Name == "trajectory-memory" || strings.HasSuffix(header.Name, "/trajectory-memory")) {
			if _, err := io.Copy(w, tr); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("binary not found in archive")
}
