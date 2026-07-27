// Package filesystem provides safe file system operations for the Agent.
// All paths are validated to prevent directory traversal attacks.
package filesystem

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AllowedRoots defines the directories where the Agent is allowed to read/write.
// Paths outside these roots are rejected.
var AllowedRoots = []string{
	"/etc/nginx/sites-available",
	"/etc/nginx/sites-enabled",
	"/etc/apache2/sites-available",
	"/etc/apache2/sites-enabled",
	"/etc/php",
	"/etc/fail2ban/jail.d",
	"/etc/fail2ban/filter.d",
	"/var/www",
	"/var/lib/opendeploy",
	"/var/log/nginx",
	"/var/log/fail2ban.log",
	"/srv",
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
	if !filepath.IsAbs(path) && !strings.HasPrefix(filepath.ToSlash(path), "/") {
		return "", fmt.Errorf("filesystem: path must be absolute")
	}
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

// securePath validates both the lexical path and every existing symlink in its
// ancestry. Missing path components are allowed for create/write operations,
// but their nearest existing parent must still resolve inside an allowed root.
func (m *Manager) securePath(path string) (string, error) {
	clean, err := m.validatePath(path)
	if err != nil {
		return "", err
	}
	existing := clean
	for {
		_, statErr := os.Lstat(existing)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("filesystem: inspect %q: %w", existing, statErr)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("filesystem: no existing parent for %q", clean)
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("filesystem: resolve %q: %w", existing, err)
	}
	if _, err := validatePathWithin(resolved, m.resolvedRoots()); err != nil {
		return "", fmt.Errorf("filesystem: symlink escapes allowed roots: %w", err)
	}
	if existing == clean {
		return resolved, nil
	}
	suffix, err := filepath.Rel(existing, clean)
	if err != nil || strings.HasPrefix(suffix, "..") {
		return "", fmt.Errorf("filesystem: invalid path suffix")
	}
	return filepath.Join(resolved, suffix), nil
}

func (m *Manager) resolvedRoots() []string {
	roots := make([]string, 0, len(m.allowedRoots))
	for _, root := range m.allowedRoots {
		resolved, err := filepath.EvalSymlinks(root)
		if err == nil {
			roots = append(roots, resolved)
		} else {
			roots = append(roots, filepath.Clean(root))
		}
	}
	return roots
}

func (m *Manager) validatePath(path string) (string, error) {
	return validatePathWithin(path, m.allowedRoots)
}

// Resolve returns a symlink-checked absolute path within an allowed root.
func (m *Manager) Resolve(path string) (string, error) {
	return m.securePath(path)
}

// Read returns the contents of a file.
func (m *Manager) Read(path string) ([]byte, error) {
	clean, err := m.securePath(path)
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
	if err := validateMode(mode); err != nil {
		return err
	}
	clean, err := m.securePath(path)
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
	clean, err := m.securePath(path)
	if err != nil {
		return err
	}
	for _, root := range m.resolvedRoots() {
		if clean == root {
			return fmt.Errorf("filesystem: refusing to delete an allowed root")
		}
	}
	if err := os.RemoveAll(clean); err != nil {
		return fmt.Errorf("filesystem: delete %q: %w", clean, err)
	}
	return nil
}

// MkdirAll creates a directory and all parents.
func (m *Manager) MkdirAll(path string, mode fs.FileMode) error {
	if err := validateMode(mode); err != nil {
		return err
	}
	clean, err := m.securePath(path)
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
	Owner   string
	Group   string
}

// List returns the direct children of a directory.
func (m *Manager) List(path string) ([]FileEntry, error) {
	clean, err := m.securePath(path)
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
		owner, group := ownership(info)

		result = append(result, FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(clean, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime().Unix(),
			Owner:   owner,
			Group:   group,
		})
	}
	return result, nil
}

// Rename renames or moves a file or directory.
func (m *Manager) Rename(oldPath, newPath string) error {
	cleanOld, err := m.securePath(oldPath)
	if err != nil {
		return err
	}
	cleanNew, err := m.securePath(newPath)
	if err != nil {
		return err
	}
	if err := os.Rename(cleanOld, cleanNew); err != nil {
		return fmt.Errorf("filesystem: rename %q to %q: %w", cleanOld, cleanNew, err)
	}
	return nil
}

// Copy copies a file from src to dst.
func (m *Manager) Copy(srcPath, dstPath string) error {
	cleanSrc, err := m.securePath(srcPath)
	if err != nil {
		return err
	}
	cleanDst, err := m.securePath(dstPath)
	if err != nil {
		return err
	}

	src, err := os.Open(cleanSrc)
	if err != nil {
		return fmt.Errorf("filesystem: open src %q: %w", cleanSrc, err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("filesystem: stat src %q: %w", cleanSrc, err)
	}

	if err := os.MkdirAll(filepath.Dir(cleanDst), 0o755); err != nil {
		return fmt.Errorf("filesystem: mkdir for dst %q: %w", cleanDst, err)
	}

	dstMode := info.Mode().Perm() &^ 0o022
	dst, err := os.OpenFile(cleanDst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, dstMode)
	if err != nil {
		return fmt.Errorf("filesystem: open dst %q: %w", cleanDst, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("filesystem: copy %q to %q: %w", cleanSrc, cleanDst, err)
	}
	return dst.Sync()
}

// Chmod changes the permissions of a file.
func (m *Manager) Chmod(path string, mode uint32) error {
	clean, err := m.securePath(path)
	if err != nil {
		return err
	}
	if err := validateMode(fs.FileMode(mode)); err != nil {
		return err
	}
	if err := os.Chmod(clean, fs.FileMode(mode).Perm()); err != nil {
		return fmt.Errorf("filesystem: chmod %q: %w", clean, err)
	}
	return nil
}

// Chown changes the owner and group of a file.
func (m *Manager) Chown(path string, uid, gid int) error {
	clean, err := m.securePath(path)
	if err != nil {
		return err
	}
	if uid <= 0 || gid <= 0 {
		return fmt.Errorf("filesystem: root or negative ownership is forbidden")
	}
	if err := os.Chown(clean, uid, gid); err != nil {
		return fmt.Errorf("filesystem: chown %q: %w", clean, err)
	}
	return nil
}

func validateMode(mode fs.FileMode) error {
	if mode.Perm() == 0 || mode&^fs.FileMode(0o777) != 0 || mode.Perm()&0o002 != 0 {
		return fmt.Errorf("filesystem: unsafe permission bits %o", mode)
	}
	return nil
}
