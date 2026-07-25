// Package filesystem provides safe file system operations for the Agent.
// All paths are validated to prevent directory traversal attacks.
package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AllowedRoots defines the directories where the Agent is allowed to read/write.
// Paths outside these roots are rejected.
var AllowedRoots = []string{
	"/etc",
	"/var/www",
	"/var/lib/opendeploy",
	"/srv",
	"/home",
	"/run/opendeploy-agent",
}

// Manager handles file system operations with path validation.
type Manager struct {
	allowedRoots []string
}

// NewManager creates a filesystem Manager.
func NewManager() *Manager {
	roots := make([]string, len(AllowedRoots))
	copy(roots, AllowedRoots)
	return &Manager{allowedRoots: roots}
}

// validatePath checks that path is under one of the allowed roots and contains
// no directory traversal sequences. Returns the cleaned path or an error.
func validatePath(path string) (string, error) {
	return validatePathWithin(path, AllowedRoots)
}

func validatePathWithin(path string, allowedRoots []string) (string, error) {
	clean := filepath.Clean(path)

	// Reject obvious traversal patterns before cleaning.
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("filesystem: path traversal detected in %q", path)
	}

	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		rel, err := filepath.Rel(cleanRoot, clean)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("filesystem: path %q is outside allowed directories", clean)
}

func (m *Manager) validatePath(path string) (string, error) {
	return validatePathWithin(path, m.allowedRoots)
}

// Read returns the contents of a file.
func (m *Manager) Read(path string) ([]byte, error) {
	clean, err := m.validatePath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", clean, err)
	}
	return data, nil
}

// Write writes content to a file, creating parent directories as needed.
func (m *Manager) Write(path string, content []byte, mode fs.FileMode) error {
	clean, err := m.validatePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		return fmt.Errorf("filesystem: mkdir for %q: %w", clean, err)
	}

	parent := filepath.Dir(clean)
	tmp, err := os.CreateTemp(parent, "."+filepath.Base(clean)+".tmp-*")
	if err != nil {
		return fmt.Errorf("filesystem: create temporary file for %q: %w", clean, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after a successful rename

	fail := func(operation string, operationErr error) error {
		_ = tmp.Close()
		return fmt.Errorf("filesystem: %s %q: %w", operation, clean, operationErr)
	}
	if err := tmp.Chmod(mode.Perm()); err != nil {
		return fail("chmod temporary file for", err)
	}
	if _, err := tmp.Write(content); err != nil {
		return fail("write temporary file for", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("sync temporary file for", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("filesystem: close temporary file for %q: %w", clean, err)
	}
	if err := os.Rename(tmpPath, clean); err != nil {
		return fmt.Errorf("filesystem: replace %q: %w", clean, err)
	}

	if err := syncDirectory(parent); err != nil {
		return fmt.Errorf("filesystem: sync parent directory for %q: %w", clean, err)
	}
	return nil
}

// Delete removes a file or directory recursively.
func (m *Manager) Delete(path string) error {
	clean, err := m.validatePath(path)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(clean); err != nil {
		return fmt.Errorf("filesystem: delete %q: %w", clean, err)
	}
	return nil
}

// MkdirAll creates a directory and all parents.
func (m *Manager) MkdirAll(path string, mode fs.FileMode) error {
	clean, err := m.validatePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(clean, mode); err != nil {
		return fmt.Errorf("filesystem: mkdir %q: %w", clean, err)
	}
	return nil
}

// FileEntry holds metadata about a single directory entry.
type FileEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	Mode    fs.FileMode
	ModTime int64 // Unix seconds
}

// List returns the direct children of a directory.
func (m *Manager) List(path string) ([]FileEntry, error) {
	clean, err := m.validatePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, fmt.Errorf("filesystem: list %q: %w", clean, err)
	}

	result := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(clean, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime().Unix(),
		})
	}
	return result, nil
}
