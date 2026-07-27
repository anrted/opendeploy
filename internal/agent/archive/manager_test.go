package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeTargetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../escape", "/etc/passwd", "a/../../escape"} {
		if _, err := safeTarget(root, name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestExtractZIPRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "bad.zip")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("owned")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), source, filepath.Join(root, "dest")); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatal("archive wrote outside destination")
	}
}

func TestExtractTARRejectsSymlink(t *testing.T) {
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	if err := writer.WriteHeader(&tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc",
	}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "bad.tar")
	if err := os.WriteFile(source, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), source, filepath.Join(root, "dest")); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
}

func TestExtractZIPWritesRegularFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "ok.zip")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("nested/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("content")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "dest")
	if err := Extract(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "nested", "file.txt"))
	if err != nil || string(content) != "content" {
		t.Fatalf("unexpected extracted file: %q, %v", content, err)
	}
}
