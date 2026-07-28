package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

type testVerifier struct{ err error }

func (v testVerifier) Verify(context.Context, string, string) error { return v.err }

type testRuntime struct {
	healthErr error
	restarts  int
}

func (r *testRuntime) Restart(context.Context) error { r.restarts++; return nil }
func (r *testRuntime) Healthy(context.Context) error { return r.healthErr }

func TestEngineAppliesSignedPinnedRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("atomic replacement semantics are verified on Linux CI")
	}
	engine, binaryDir, server := testEngine(t, nil)
	defer server.Close()
	entry, err := engine.Apply(context.Background(), "v1.2.3", "v1.2.2")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if entry.Status != "succeeded" {
		t.Fatalf("status = %q", entry.Status)
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		content, err := os.ReadFile(filepath.Join(binaryDir, name))
		if err != nil || string(content) != "new-"+name {
			t.Fatalf("%s = %q, %v", name, content, err)
		}
	}
	history, err := engine.History()
	if err != nil || len(history) != 1 || history[0].Status != "succeeded" {
		t.Fatalf("history = %#v, %v", history, err)
	}
}

func TestEngineAutomaticallyRollsBackFailedHealthGate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("atomic replacement semantics are verified on Linux CI")
	}
	engine, binaryDir, server := testEngine(t, errors.New("unhealthy"))
	defer server.Close()
	entry, err := engine.Apply(context.Background(), "v1.2.3", "v1.2.2")
	if err == nil || entry.Status != "rolled_back" || !entry.Automatic {
		t.Fatalf("entry = %#v, err = %v", entry, err)
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		content, readErr := os.ReadFile(filepath.Join(binaryDir, name))
		if readErr != nil || string(content) != "old-"+name {
			t.Fatalf("%s was not rolled back: %q, %v", name, content, readErr)
		}
	}
}

func TestEngineSupportsManualRollbackFromJournal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("atomic replacement semantics are verified on Linux CI")
	}
	engine, binaryDir, server := testEngine(t, nil)
	defer server.Close()
	applied, err := engine.Apply(context.Background(), "v1.2.3", "v1.2.2")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	rolledBack, err := engine.Rollback(context.Background(), applied.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.Status != "manual_rollback" {
		t.Fatalf("status = %q", rolledBack.Status)
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		content, readErr := os.ReadFile(filepath.Join(binaryDir, name))
		if readErr != nil || string(content) != "old-"+name {
			t.Fatalf("%s was not manually rolled back: %q, %v", name, content, readErr)
		}
	}
	history, err := engine.History()
	if err != nil || len(history) != 2 || history[1].Status != "manual_rollback" {
		t.Fatalf("history = %#v, %v", history, err)
	}
}

func TestFailedManualRollbackRestoresCurrentVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("atomic replacement semantics are verified on Linux CI")
	}
	engine, binaryDir, server := testEngine(t, nil)
	defer server.Close()
	applied, err := engine.Apply(context.Background(), "v1.2.3", "v1.2.2")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	engine.Runtime.(*testRuntime).healthErr = errors.New("previous version unhealthy")
	rolledBack, err := engine.Rollback(context.Background(), applied.ID)
	if err == nil || rolledBack.Status != "rollback_failed" {
		t.Fatalf("entry = %#v, err = %v", rolledBack, err)
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		content, readErr := os.ReadFile(filepath.Join(binaryDir, name))
		if readErr != nil || string(content) != "new-"+name {
			t.Fatalf("%s current version was not restored: %q, %v", name, content, readErr)
		}
	}
}

func TestEngineRejectsInvalidSignatureBeforeInstallation(t *testing.T) {
	engine, binaryDir, server := testEngine(t, nil)
	defer server.Close()
	engine.Verifier = testVerifier{err: errors.New("bad signature")}
	if _, err := engine.Apply(context.Background(), "v1.2.3", "v1.2.2"); err == nil {
		t.Fatal("invalid signature was accepted")
	}
	content, _ := os.ReadFile(filepath.Join(binaryDir, "opendeploy-core"))
	if string(content) != "old-opendeploy-core" {
		t.Fatal("binary changed before signature verification")
	}
}

func testEngine(t *testing.T, healthErr error) (*Engine, string, *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	binaryDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		if err := os.WriteFile(filepath.Join(binaryDir, name), []byte("old-"+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	archive := releaseArchive(t)
	digest := sha256.Sum256(archive)
	manifest := Manifest{
		Schema: ManifestSchema, Version: "v1.2.3", Tag: "v1.2.3", Commit: testCommit,
		PublishedAt: time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC),
		Artifacts: []Artifact{{
			OS: runtime.GOOS, Arch: runtime.GOARCH, Name: "opendeploy-test.tar.gz",
			SHA256: hex.EncodeToString(digest[:]), Size: int64(len(archive)),
		}},
	}
	manifestData, _ := json.Marshal(manifest)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/anrted/opendeploy/releases/tags/v1.2.3":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"tag_name": "v1.2.3", "draft": false, "prerelease": false,
				"assets": []map[string]string{
					{"name": manifestAsset, "browser_download_url": server.URL + "/assets/" + manifestAsset},
					{"name": sigstoreAsset, "browser_download_url": server.URL + "/assets/" + sigstoreAsset},
					{"name": "opendeploy-test.tar.gz", "browser_download_url": server.URL + "/assets/opendeploy-test.tar.gz"},
				},
			})
		case "/repos/anrted/opendeploy/git/ref/tags/v1.2.3":
			_ = json.NewEncoder(response).Encode(map[string]any{"object": map[string]string{"type": "commit", "sha": testCommit}})
		case "/assets/" + manifestAsset:
			_, _ = response.Write(manifestData)
		case "/assets/" + sigstoreAsset:
			_, _ = response.Write([]byte("test bundle"))
		case "/assets/opendeploy-test.tar.gz":
			_, _ = response.Write(archive)
		default:
			http.NotFound(response, request)
		}
	}))
	client := server.Client()
	github := NewGitHubClient(client)
	github.APIBase = server.URL
	config := Config{
		StateDir: filepath.Join(root, "state"), ReleaseDir: filepath.Join(root, "releases"),
		BinaryDir: binaryDir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Signature: sigstoreAsset, HealthTimeout: time.Second,
	}
	engine := NewEngine(config, github, testVerifier{}, &testRuntime{healthErr: healthErr})
	engine.Client = client
	return engine, binaryDir, server
}

func releaseArchive(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy-cli"} {
		target := name
		if name == "opendeploy-cli" {
			target = "opendeploy"
		}
		content := []byte("new-" + target)
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
