package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePath checks that path is under one of the allowed roots and contains
// no directory traversal sequences.
func validatePath(path string) (string, error) {
	return validatePathWithin(path, AllowedRoots)
}

func validatePathWithin(path string, allowedRoots []string) (string, error) {
	if !filepath.IsAbs(path) && !strings.HasPrefix(filepath.ToSlash(path), "/") {
		return "", fmt.Errorf("filesystem: path must be absolute")
	}
	clean := filepath.Clean(path)
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
// ancestry. Missing components are allowed if their existing parent is safe.
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
