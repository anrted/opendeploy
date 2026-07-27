package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidatePathHonorsRootBoundary(t *testing.T) {
	if _, err := validatePath("/etc/nginx/sites-available/example.conf"); err != nil {
		t.Fatalf("expected managed nginx site path to be allowed: %v", err)
	}
	for _, path := range []string{"/etc/passwd", "/etc/systemd/system/evil.service", "/home/root/.ssh/authorized_keys", "/var/www2/site", "/etc/nginx/sites-available/../../../tmp/file"} {
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

func TestSecurePathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires additional privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{allowedRoots: []string{root}}
	if _, err := manager.Resolve(filepath.Join(link, "secret")); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestDeleteRefusesAllowedRoot(t *testing.T) {
	root := t.TempDir()
	manager := &Manager{allowedRoots: []string{root}}
	if err := manager.Delete(root); err == nil {
		t.Fatal("expected root deletion to be rejected")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("allowed root was removed: %v", err)
	}
}

func TestChmodRejectsSpecialBits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{allowedRoots: []string{root}}
	if err := manager.Chmod(path, 0o4755); err == nil {
		t.Fatal("expected setuid mode to be rejected")
	}
}
