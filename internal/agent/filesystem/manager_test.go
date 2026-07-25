package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidatePathHonorsRootBoundary(t *testing.T) {
	if _, err := validatePath("/etc/nginx/nginx.conf"); err != nil {
		t.Fatalf("expected path below /etc to be allowed: %v", err)
	}
	for _, path := range []string{"/etc-shadow/passwd", "/var/www2/site", "/etc/../tmp/file"} {
		if _, err := validatePath(path); err == nil {
			t.Errorf("expected %q to be rejected", path)
		}
	}
}

func TestWriteAtomicallyReplacesFileAndPreservesRequestedMode(t *testing.T) {
	root := t.TempDir()
	manager := &Manager{allowedRoots: []string{root}}
	path := filepath.Join(root, "nginx", "sites-available", "example.conf")

	if err := manager.Write(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write initial content: %v", err)
	}
	if err := manager.Write(path, []byte("new configuration"), 0o640); err != nil {
		t.Fatalf("replace content: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if got, want := string(content), "new configuration"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat replaced file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
			t.Fatalf("mode = %o, want %o", got, want)
		}
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", matches)
	}
}
