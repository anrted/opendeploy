package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	manifestAsset = "release-manifest.json"
	sigstoreAsset = "release-manifest.json.bundle"
	maxMetadata   = 4 << 20
)

type Config struct {
	StateDir      string
	ReleaseDir    string
	BinaryDir     string
	GOOS          string
	GOARCH        string
	Signature     string
	HealthTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		StateDir: "/var/lib/opendeploy/updates", ReleaseDir: "/opt/opendeploy/releases",
		BinaryDir: "/usr/bin", Signature: sigstoreAsset, HealthTimeout: 45 * time.Second,
	}
}

type Engine struct {
	Config   Config
	GitHub   *GitHubClient
	Verifier SignatureVerifier
	Runtime  RuntimeController
	Client   *http.Client
	Now      func() time.Time
}

type HistoryEntry struct {
	ID          string    `json:"id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	FromVersion string    `json:"from_version,omitempty"`
	ToVersion   string    `json:"to_version"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	Automatic   bool      `json:"automatic_rollback,omitempty"`
	Error       string    `json:"error,omitempty"`
	BackupDir   string    `json:"backup_dir,omitempty"`
}

func NewEngine(config Config, github *GitHubClient, verifier SignatureVerifier, runtime RuntimeController) *Engine {
	client := secureHTTPClient(2 * time.Minute)
	if github == nil {
		github = NewGitHubClient(client)
	}
	return &Engine{Config: config, GitHub: github, Verifier: verifier, Runtime: runtime, Client: client, Now: time.Now}
}

func (e *Engine) Apply(ctx context.Context, tag, currentVersion string) (entry HistoryEntry, err error) {
	if e.Verifier == nil || e.Runtime == nil {
		return entry, fmt.Errorf("update: verifier and runtime controller are required")
	}
	if err := e.prepareDirectories(); err != nil {
		return entry, err
	}
	unlock, err := acquireUpdateLock(filepath.Join(e.Config.StateDir, "update.lock"))
	if err != nil {
		return entry, err
	}
	defer unlock()
	requestedAt := e.Now().UTC()
	entry = HistoryEntry{
		ID: requestedAt.Format("20060102T150405.000000000Z"), StartedAt: requestedAt,
		FromVersion: currentVersion, ToVersion: tag, Status: "verifying",
	}
	defer func() {
		if err != nil && entry.CompletedAt.IsZero() {
			entry.Status, entry.Error, entry.CompletedAt = "failed", err.Error(), e.Now().UTC()
			_ = e.appendHistory(entry)
		}
	}()
	release, err := e.GitHub.Release(ctx, tag)
	if err != nil {
		return entry, err
	}
	manifestURL, ok := release.Assets[manifestAsset]
	if !ok {
		return entry, fmt.Errorf("update: release manifest asset is missing")
	}
	signatureName := e.Config.Signature
	if signatureName == "" {
		signatureName = sigstoreAsset
	}
	signatureURL, ok := release.Assets[signatureName]
	if !ok {
		return entry, fmt.Errorf("update: manifest signature asset %q is missing", signatureName)
	}

	stage, err := os.MkdirTemp(filepath.Join(e.Config.StateDir, "staging"), "update-*")
	if err != nil {
		return entry, fmt.Errorf("update: create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	manifestPath := filepath.Join(stage, manifestAsset)
	signaturePath := filepath.Join(stage, filepath.Base(signatureName))
	if err := e.download(ctx, manifestURL, manifestPath, maxMetadata); err != nil {
		return entry, err
	}
	if err := e.download(ctx, signatureURL, signaturePath, maxMetadata); err != nil {
		return entry, err
	}
	if err := e.Verifier.Verify(ctx, manifestPath, signaturePath); err != nil {
		return entry, err
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return entry, err
	}
	manifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return entry, err
	}
	if manifest.Tag != release.Tag {
		return entry, fmt.Errorf("update: signed manifest tag does not match GitHub release")
	}
	tagCommit, err := e.GitHub.TagCommit(ctx, release.Tag)
	if err != nil {
		return entry, err
	}
	if tagCommit != manifest.Commit {
		return entry, fmt.Errorf("update: signed manifest commit does not match release tag")
	}
	entry.Commit = manifest.Commit
	entry.ID += "-" + manifest.Commit[:12]
	artifact, err := e.artifact(manifest)
	if err != nil {
		return entry, err
	}
	artifactURL, ok := release.Assets[artifact.Name]
	if !ok {
		return entry, fmt.Errorf("update: signed artifact %q is missing from release", artifact.Name)
	}
	archivePath := filepath.Join(stage, artifact.Name)
	if err := e.download(ctx, artifactURL, archivePath, artifact.Size); err != nil {
		return entry, err
	}
	if info, err := os.Stat(archivePath); err != nil || info.Size() != artifact.Size {
		return entry, fmt.Errorf("update: artifact size does not match signed manifest")
	}
	if err := verifySHA256(archivePath, artifact.SHA256); err != nil {
		return entry, err
	}
	payloadDir := filepath.Join(stage, "payload")
	if err := extractRelease(archivePath, payloadDir); err != nil {
		return entry, err
	}

	entry.Status = "installing"
	entry.BackupDir = filepath.Join(e.Config.StateDir, "backups", entry.ID)
	if err := e.install(payloadDir, manifest, entry.BackupDir); err != nil {
		rollbackErr := e.restore(entry.BackupDir)
		combined := errors.Join(err, rollbackErr)
		entry.Status, entry.Automatic, entry.Error, entry.CompletedAt =
			"rolled_back", true, combined.Error(), e.Now().UTC()
		_ = e.appendHistory(entry)
		return entry, fmt.Errorf("update: installation failed and rollback executed: %w", combined)
	}
	healthCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.healthTimeout())
	defer cancel()
	if err := e.Runtime.Restart(healthCtx); err == nil {
		err = e.Runtime.Healthy(healthCtx)
	}
	if err != nil {
		rollbackErr := e.restore(entry.BackupDir)
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), e.healthTimeout())
		defer rollbackCancel()
		restartErr := e.Runtime.Restart(rollbackCtx)
		healthErr := e.Runtime.Healthy(rollbackCtx)
		combined := errors.Join(err, rollbackErr, restartErr, healthErr)
		entry.Status, entry.Automatic, entry.Error = "rolled_back", true, combined.Error()
		entry.CompletedAt = e.Now().UTC()
		_ = e.appendHistory(entry)
		return entry, fmt.Errorf("update: health gate failed and rollback executed: %w", combined)
	}
	entry.Status, entry.CompletedAt = "succeeded", e.Now().UTC()
	if err := e.appendHistory(entry); err != nil {
		return entry, err
	}
	return entry, nil
}

func (e *Engine) Rollback(ctx context.Context, transactionID string) (HistoryEntry, error) {
	if err := e.prepareDirectories(); err != nil {
		return HistoryEntry{}, err
	}
	unlock, err := acquireUpdateLock(filepath.Join(e.Config.StateDir, "update.lock"))
	if err != nil {
		return HistoryEntry{}, err
	}
	defer unlock()
	original, err := e.findHistory(transactionID)
	if err != nil {
		return HistoryEntry{}, err
	}
	if original.BackupDir == "" {
		return HistoryEntry{}, fmt.Errorf("update: transaction has no rollback snapshot")
	}
	now := e.Now().UTC()
	entry := HistoryEntry{
		ID: "rollback-" + now.Format("20060102T150405.000000000Z"), StartedAt: now,
		FromVersion: original.ToVersion, ToVersion: original.FromVersion, Commit: original.Commit,
		Status: "rolling_back", BackupDir: original.BackupDir,
	}
	safetySnapshot := filepath.Join(e.Config.StateDir, "backups", entry.ID)
	if err := e.snapshotLive(safetySnapshot); err != nil {
		entry.Status, entry.Error, entry.CompletedAt = "rollback_failed", err.Error(), e.Now().UTC()
		_ = e.appendHistory(entry)
		return entry, err
	}
	if err := e.restore(original.BackupDir); err != nil {
		entry.Status, entry.Error, entry.CompletedAt = "rollback_failed", err.Error(), e.Now().UTC()
		_ = e.appendHistory(entry)
		return entry, err
	}
	healthCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.healthTimeout())
	defer cancel()
	runtimeErr := e.Runtime.Restart(healthCtx)
	if runtimeErr == nil {
		runtimeErr = e.Runtime.Healthy(healthCtx)
	}
	if runtimeErr != nil {
		restoreErr := e.restore(safetySnapshot)
		recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), e.healthTimeout())
		defer recoveryCancel()
		restartErr := e.Runtime.Restart(recoveryCtx)
		healthErr := e.Runtime.Healthy(recoveryCtx)
		combined := errors.Join(runtimeErr, restoreErr, restartErr, healthErr)
		entry.Status, entry.Error, entry.CompletedAt = "rollback_failed", combined.Error(), e.Now().UTC()
		_ = e.appendHistory(entry)
		return entry, fmt.Errorf("update: rollback health gate failed; current version restored: %w", combined)
	}
	entry.Status, entry.CompletedAt = "manual_rollback", e.Now().UTC()
	return entry, e.appendHistory(entry)
}

func (e *Engine) History() ([]HistoryEntry, error) {
	data, err := os.ReadFile(filepath.Join(e.Config.StateDir, "history.jsonl"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []HistoryEntry
	for lineNumber, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("update: corrupt history line %d: %w", lineNumber+1, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (e *Engine) artifact(manifest *Manifest) (Artifact, error) {
	if e.Config.GOOS != "" || e.Config.GOARCH != "" {
		return manifest.ArtifactFor(e.Config.GOOS, e.Config.GOARCH)
	}
	return manifest.ArtifactForCurrentPlatform()
}

func (e *Engine) prepareDirectories() error {
	for _, directory := range []string{
		e.Config.StateDir, filepath.Join(e.Config.StateDir, "staging"),
		filepath.Join(e.Config.StateDir, "backups"), e.Config.ReleaseDir,
	} {
		if directory == "" || !filepath.IsAbs(directory) {
			return fmt.Errorf("update: all configured directories must be absolute")
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("update: create directory %s: %w", directory, err)
		}
	}
	return nil
}

func (e *Engine) download(ctx context.Context, sourceURL, destination string, limit int64) error {
	if limit <= 0 || limit > 512<<20 {
		return fmt.Errorf("update: invalid download limit")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	response, err := e.Client.Do(request)
	if err != nil {
		return fmt.Errorf("update: download %s: %w", filepath.Base(destination), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download %s returned HTTP %d", filepath.Base(destination), response.StatusCode)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return errors.Join(copyErr, closeErr)
	}
	if written > limit {
		return fmt.Errorf("update: download %s exceeds signed size limit", filepath.Base(destination))
	}
	return nil
}

func (e *Engine) install(payload string, manifest *Manifest, backupDir string) error {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	releaseDir := filepath.Join(e.Config.ReleaseDir,
		strings.TrimPrefix(manifest.Version, "v")+"-"+manifest.Commit[:12]+"-"+filepath.Base(backupDir))
	if err := os.Mkdir(releaseDir, 0o755); err != nil {
		return err
	}
	names := map[string]string{"opendeploy-core": "opendeploy-core", "opendeploy-agent": "opendeploy-agent", "opendeploy-cli": "opendeploy"}
	// Prepare the complete immutable release and rollback snapshot before
	// replacing the first live binary.
	for sourceName, targetName := range names {
		source := filepath.Join(payload, sourceName)
		if err := copyFile(source, filepath.Join(releaseDir, targetName), 0o755); err != nil {
			return err
		}
		target := filepath.Join(e.Config.BinaryDir, targetName)
		if _, err := os.Stat(target); err != nil {
			return fmt.Errorf("update: live binary %s is unavailable: %w", targetName, err)
		}
		if err := copyFile(target, filepath.Join(backupDir, targetName), 0o755); err != nil {
			return err
		}
	}
	for _, targetName := range names {
		target := filepath.Join(e.Config.BinaryDir, targetName)
		if err := atomicCopy(filepath.Join(releaseDir, targetName), target, 0o755); err != nil {
			return err
		}
	}
	state, _ := json.Marshal(map[string]string{"version": manifest.Version, "commit": manifest.Commit})
	return os.WriteFile(filepath.Join(e.Config.StateDir, "current.json"), state, 0o600)
}

func (e *Engine) restore(backupDir string) error {
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		backup := filepath.Join(backupDir, name)
		if _, err := os.Stat(backup); err != nil {
			return fmt.Errorf("update: rollback snapshot missing %s: %w", name, err)
		}
		if err := atomicCopy(backup, filepath.Join(e.Config.BinaryDir, name), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) snapshotLive(destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"opendeploy-core", "opendeploy-agent", "opendeploy"} {
		if err := copyFile(filepath.Join(e.Config.BinaryDir, name), filepath.Join(destination, name), 0o755); err != nil {
			return fmt.Errorf("update: snapshot live binary %s: %w", name, err)
		}
	}
	return nil
}

func (e *Engine) appendHistory(entry HistoryEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(e.Config.StateDir, "history.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (e *Engine) findHistory(id string) (HistoryEntry, error) {
	entries, err := e.History()
	if err != nil {
		return HistoryEntry{}, err
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if (id == "" && entries[index].Status == "succeeded") || entries[index].ID == id {
			return entries[index], nil
		}
	}
	return HistoryEntry{}, fmt.Errorf("update: rollback transaction not found")
}

func (e *Engine) healthTimeout() time.Duration {
	if e.Config.HealthTimeout <= 0 {
		return 45 * time.Second
	}
	return e.Config.HealthTimeout
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != strings.ToLower(expected) {
		return fmt.Errorf("update: SHA256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

func extractRelease(archivePath, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("update: open release archive: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	expected := map[string]bool{"opendeploy-core": false, "opendeploy-agent": false, "opendeploy-cli": false}
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("update: read release archive: %w", err)
		}
		name := strings.TrimPrefix(filepath.ToSlash(header.Name), "./")
		if _, ok := expected[name]; !ok || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > 256<<20 {
			return fmt.Errorf("update: unexpected archive entry %q", header.Name)
		}
		total += header.Size
		if total > 512<<20 {
			return fmt.Errorf("update: extracted release exceeds size limit")
		}
		target := filepath.Join(destination, name)
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(output, reader, header.Size)
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		expected[name] = true
	}
	missing := make([]string, 0)
	for name, found := range expected {
		if !found {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("update: release archive is missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

func atomicCopy(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".update-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	input, err := os.Open(source)
	if err != nil {
		_ = temp.Close()
		return err
	}
	_, copyErr := io.Copy(temp, input)
	closeInputErr := input.Close()
	chmodErr := temp.Chmod(mode)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err := errors.Join(copyErr, closeInputErr, chmodErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return fmt.Errorf("update: atomically replace %s: %w", destination, err)
	}
	return nil
}
