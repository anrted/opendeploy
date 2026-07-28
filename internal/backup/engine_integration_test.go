package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type testRestoreRuntime struct {
	afterCalls int
	failFirst  bool
}

func (*testRestoreRuntime) BeforeRestore(context.Context) error { return nil }
func (r *testRestoreRuntime) AfterRestore(context.Context) error {
	r.afterCalls++
	if r.failFirst && r.afterCalls == 1 {
		return errors.New("unhealthy")
	}
	return nil
}

func TestCreateVerifyAndRestoreOnCleanServer(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	writeTestFile(t, filepath.Join(sourceRoot, "panel", "config.yaml"), "panel-config")
	writeTestFile(t, filepath.Join(sourceRoot, "sites", "example", "index.html"), "site-data")
	createConfig := testConfig(t, sourceRoot)
	createEngine := NewEngine(createConfig, "v1.2.3")
	createEngine.Now = func() time.Time { return time.Date(2026, 7, 29, 1, 2, 3, 0, time.UTC) }

	manifest, archivePath, err := createEngine.Create(context.Background(), "integration-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if manifest.Schema != Schema || len(manifest.Entries) != 2 {
		t.Fatalf("manifest = %#v", manifest)
	}
	verified, err := createEngine.Verify(context.Background(), archivePath)
	if err != nil || verified.ID != manifest.ID {
		t.Fatalf("Verify: %#v, %v", verified, err)
	}

	restoreConfig := testConfig(t, targetRoot)
	restoreConfig.BackupDir = createConfig.BackupDir
	restoreEngine := NewEngine(restoreConfig, "clean")
	restored, err := restoreEngine.Restore(context.Background(), archivePath)
	if err != nil || restored.ID != manifest.ID {
		t.Fatalf("Restore: %#v, %v", restored, err)
	}
	assertTestFile(t, filepath.Join(targetRoot, "panel", "config.yaml"), "panel-config")
	assertTestFile(t, filepath.Join(targetRoot, "sites", "example", "index.html"), "site-data")
	history, err := restoreEngine.History()
	if err != nil || len(history) != 1 || history[0].Status != "succeeded" {
		t.Fatalf("history = %#v, %v", history, err)
	}
}

func TestVerifyRejectsModifiedPayload(t *testing.T) {
	sourceRoot := t.TempDir()
	writeTestFile(t, filepath.Join(sourceRoot, "panel", "config.yaml"), "original")
	config := testConfig(t, sourceRoot)
	config.Sources = config.Sources[:1]
	engine := NewEngine(config, "test")
	_, archivePath, err := engine.Create(context.Background(), "tamper-test")
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.tar.gz")
	tamperArchive(t, archivePath, tampered)
	if _, err := engine.Verify(context.Background(), tampered); err == nil ||
		!strings.Contains(err.Error(), "integrity check failed") {
		t.Fatalf("tampered backup accepted: %v", err)
	}
}

func TestRestoreRollsBackFilesWhenHealthGateFails(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	writeTestFile(t, filepath.Join(sourceRoot, "panel", "config.yaml"), "backup-value")
	createConfig := testConfig(t, sourceRoot)
	createConfig.Sources = createConfig.Sources[:1]
	createEngine := NewEngine(createConfig, "test")
	_, archivePath, err := createEngine.Create(context.Background(), "health-test")
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(targetRoot, "panel", "config.yaml"), "current-value")
	restoreConfig := testConfig(t, targetRoot)
	restoreConfig.Sources = restoreConfig.Sources[:1]
	restoreEngine := NewEngine(restoreConfig, "test")
	runtime := &testRestoreRuntime{failFirst: true}
	restoreEngine.Runtime = runtime
	if _, err := restoreEngine.Restore(context.Background(), archivePath); err == nil {
		t.Fatal("restore health failure was accepted")
	}
	assertTestFile(t, filepath.Join(targetRoot, "panel", "config.yaml"), "current-value")
	if runtime.afterCalls != 2 {
		t.Fatalf("recovery restart calls = %d, want 2", runtime.afterCalls)
	}
}

func TestDatabaseSnapshotIsConsistentAndRestorable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SQLite CGO integration runs on Linux CI")
	}
	sourceRoot := t.TempDir()
	databasePath := filepath.Join(sourceRoot, "data.db")
	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE sample (value TEXT); INSERT INTO sample VALUES ('preserved')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	config := testConfig(t, sourceRoot)
	config.Sources = []Source{{ID: "database", Path: databasePath, Required: true, Database: true}}
	engine := NewEngine(config, "test")
	_, archivePath, err := engine.Create(context.Background(), "database-test")
	if err != nil {
		t.Fatal(err)
	}

	targetRoot := t.TempDir()
	targetDB := filepath.Join(targetRoot, "data.db")
	restoreConfig := testConfig(t, targetRoot)
	restoreConfig.Sources = []Source{{ID: "database", Path: targetDB, Required: true, Database: true}}
	restoreConfig.BackupDir = config.BackupDir
	if _, err := NewEngine(restoreConfig, "test").Restore(context.Background(), archivePath); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite3", targetDB+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restored.Close() }()
	var value string
	if err := restored.QueryRow(`SELECT value FROM sample`).Scan(&value); err != nil || value != "preserved" {
		t.Fatalf("restored value = %q, err = %v", value, err)
	}
}

func TestInternalCertificateSymlinkIsCapturedAsPortableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics are verified on Linux CI")
	}
	sourceRoot := t.TempDir()
	writeTestFile(t, filepath.Join(sourceRoot, "ssl", "archive", "cert.pem"), "certificate")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "ssl", "live"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "archive", "cert.pem"), filepath.Join(sourceRoot, "ssl", "live", "cert.pem")); err != nil {
		t.Fatal(err)
	}
	config := testConfig(t, sourceRoot)
	config.Sources = []Source{{ID: "ssl", Path: filepath.Join(sourceRoot, "ssl"), Required: true}}
	engine := NewEngine(config, "test")
	_, archivePath, err := engine.Create(context.Background(), "ssl-test")
	if err != nil {
		t.Fatal(err)
	}
	targetRoot := t.TempDir()
	restoreConfig := testConfig(t, targetRoot)
	restoreConfig.Sources = []Source{{ID: "ssl", Path: filepath.Join(targetRoot, "ssl"), Required: true}}
	restoreConfig.BackupDir = config.BackupDir
	if _, err := NewEngine(restoreConfig, "test").Restore(context.Background(), archivePath); err != nil {
		t.Fatal(err)
	}
	assertTestFile(t, filepath.Join(targetRoot, "ssl", "live", "cert.pem"), "certificate")
}

func testConfig(t *testing.T, root string) Config {
	t.Helper()
	return Config{
		BackupDir:  filepath.Join(root, "backups"),
		StateDir:   filepath.Join(root, "state"),
		MaxEntries: 100,
		MaxBytes:   10 << 20,
		Sources: []Source{
			{ID: "panel", Path: filepath.Join(root, "panel"), Required: true},
			{ID: "sites", Path: filepath.Join(root, "sites")},
		},
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test temp path
	if err != nil || string(data) != expected {
		t.Fatalf("%s = %q, %v", path, data, err)
	}
}

func tamperArchive(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source) //nolint:gosec // test temp path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close() }()
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(destination) //nolint:gosec // test temp path
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarReader, tarWriter := tar.NewReader(gzipReader), tar.NewWriter(gzipWriter)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		data, readErr := io.ReadAll(tarReader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.HasPrefix(header.Name, "data/") && len(data) > 0 {
			data[0] ^= 0xff
		}
		header.Size = int64(len(data))
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
