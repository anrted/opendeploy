package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const manifestName = "manifest.json"

type Engine struct {
	Config  Config
	Now     func() time.Time
	Version string
	Runtime RestoreRuntime
}

type collectedFile struct {
	entry Entry
	path  string
}

func NewEngine(config Config, version string) *Engine {
	return &Engine{Config: config, Version: version, Now: time.Now}
}

// CreateBackup adapts Engine to critical-operation backup hooks.
func (e *Engine) CreateBackup(ctx context.Context, reason string) (string, error) {
	_, archivePath, err := e.Create(ctx, reason)
	return archivePath, err
}

// Create keeps collection, publication and post-write verification in one
// auditable transaction.
func (e *Engine) Create(ctx context.Context, reason string) (manifest Manifest, archivePath string, err error) { //nolint:gocyclo
	if err := e.prepare(); err != nil {
		return manifest, "", err
	}
	now := e.Now().UTC()
	manifest = Manifest{
		Schema: Schema, ID: now.Format("20060102T150405.000000000Z"),
		CreatedAt: now, Reason: strings.TrimSpace(reason), OpenDeploy: e.Version,
	}
	if manifest.Reason == "" {
		manifest.Reason = "manual"
	}
	operation := Operation{
		ID: "create-" + manifest.ID, Type: "create", BackupID: manifest.ID,
		Reason: manifest.Reason, Status: "running", StartedAt: now,
	}
	defer func() {
		operation.FinishedAt = e.Now().UTC()
		if err != nil {
			operation.Status, operation.Error = "failed", err.Error()
		} else {
			operation.Status, operation.Archive = "succeeded", archivePath
		}
		_ = e.appendHistory(operation)
	}()

	stage, err := os.MkdirTemp(e.Config.StateDir, "create-*")
	if err != nil {
		return manifest, "", fmt.Errorf("backup: create staging: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()
	files, err := e.collect(ctx, stage)
	if err != nil {
		return manifest, "", err
	}
	for _, file := range files {
		manifest.Entries = append(manifest.Entries, file.entry)
		manifest.TotalBytes += file.entry.Size
	}
	archivePath = filepath.Join(e.Config.BackupDir, "opendeploy-"+manifest.ID+".tar.gz")
	temp, err := os.CreateTemp(e.Config.BackupDir, ".backup-*.tar.gz")
	if err != nil {
		return manifest, "", fmt.Errorf("backup: create archive: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := writeArchive(temp, manifest, files); err != nil {
		_ = temp.Close()
		return manifest, "", err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return manifest, "", fmt.Errorf("backup: sync archive: %w", err)
	}
	if err := temp.Close(); err != nil {
		return manifest, "", fmt.Errorf("backup: close archive: %w", err)
	}
	if err := os.Rename(tempPath, archivePath); err != nil {
		return manifest, "", fmt.Errorf("backup: publish archive: %w", err)
	}
	if _, _, err := e.extractVerified(ctx, archivePath, ""); err != nil {
		_ = os.Remove(archivePath)
		return manifest, "", fmt.Errorf("backup: verify newly created archive: %w", err)
	}
	return manifest, archivePath, nil
}

func (e *Engine) collect(ctx context.Context, stage string) ([]collectedFile, error) { //nolint:gocyclo
	var files []collectedFile
	for _, source := range e.Config.Sources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		path := source.Path
		if source.Database {
			snapshot := filepath.Join(stage, source.ID+".db")
			if err := snapshotSQLite(ctx, source.Path, snapshot); err != nil {
				if source.Required {
					return nil, err
				}
				continue
			}
			path = snapshot
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) && !source.Required {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("backup: inspect %s: %w", source.ID, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("backup: source %s must not be a symlink", source.ID)
		}
		if !info.IsDir() {
			file, err := collectOne(source.ID, filepath.Base(source.Path), path, info)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
			continue
		}
		err = filepath.Walk(path, func(current string, currentInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if current == path {
				return nil
			}
			if currentInfo.Mode()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(current)
				if err != nil {
					return err
				}
				relativeTarget, err := filepath.Rel(path, resolved)
				if err != nil || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
					// External system links (common in systemd unit directories)
					// are owned by another source/package and are not followed.
					return nil
				}
				targetInfo, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				if !targetInfo.Mode().IsRegular() {
					return nil
				}
				relative, err := filepath.Rel(path, current)
				if err != nil {
					return err
				}
				file, err := collectOne(source.ID, filepath.ToSlash(relative), resolved, targetInfo)
				if err == nil {
					files = append(files, file)
				}
				return err
			}
			if !currentInfo.Mode().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(path, current)
			if err != nil {
				return err
			}
			file, err := collectOne(source.ID, filepath.ToSlash(relative), current, currentInfo)
			if err == nil {
				files = append(files, file)
			}
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("backup: collect %s: %w", source.ID, err)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].entry.SourceID+"/"+files[i].entry.Path < files[j].entry.SourceID+"/"+files[j].entry.Path
	})
	return files, nil
}

func collectOne(sourceID, relative, path string, info os.FileInfo) (collectedFile, error) {
	file, err := os.Open(path) //nolint:gosec // path comes from an administrator-configured source
	if err != nil {
		return collectedFile{}, err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return collectedFile{}, err
	}
	return collectedFile{
		entry: Entry{
			SourceID: sourceID, Path: relative, SHA256: hex.EncodeToString(hash.Sum(nil)),
			Size: size, Mode: uint32(info.Mode().Perm()),
		},
		path: path,
	}, nil
}

func writeArchive(output io.Writer, manifest Manifest, files []collectedFile) error {
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := writeTarBytes(tarWriter, manifestName, data, 0o600); err != nil {
		return err
	}
	for _, item := range files {
		if err := writeTarFile(tarWriter, item); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("backup: close tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("backup: close gzip: %w", err)
	}
	return nil
}

func writeTarBytes(writer *tar.Writer, name string, data []byte, mode int64) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func writeTarFile(writer *tar.Writer, item collectedFile) error {
	input, err := os.Open(item.path) //nolint:gosec // collected path was validated before archiving
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	name := "data/" + item.entry.SourceID + "/" + item.entry.Path
	if err := writer.WriteHeader(&tar.Header{
		Name: name, Mode: int64(item.entry.Mode), Size: item.entry.Size, Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	written, err := io.Copy(writer, input)
	if err != nil || written != item.entry.Size {
		return fmt.Errorf("backup: archive %s: %w", name, errors.Join(err, io.ErrUnexpectedEOF))
	}
	return nil
}

func snapshotSQLite(ctx context.Context, source, destination string) error {
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("backup: database source: %w", err)
	}
	db, err := sql.Open("sqlite3", source+"?_busy_timeout=10000")
	if err != nil {
		return fmt.Errorf("backup: open database: %w", err)
	}
	defer func() { _ = db.Close() }()
	escaped := strings.ReplaceAll(destination, "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil { //nolint:gosec // SQLite has no parameter binding for VACUUM INTO; quotes are escaped
		return fmt.Errorf("backup: consistent database snapshot: %w", err)
	}
	check, err := sql.Open("sqlite3", destination+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = check.Close() }()
	var result string
	if err := check.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil || result != "ok" {
		return fmt.Errorf("backup: database integrity check failed: %s: %w", result, err)
	}
	return nil
}

func (e *Engine) prepare() error {
	if err := e.Config.validate(); err != nil {
		return err
	}
	for _, directory := range []string{e.Config.BackupDir, e.Config.StateDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("backup: create directory: %w", err)
		}
	}
	return nil
}

func (e *Engine) appendHistory(operation Operation) error {
	if err := os.MkdirAll(e.Config.StateDir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(e.Config.StateDir, "history.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // state root is operator-configured and private
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (e *Engine) History() ([]Operation, error) {
	data, err := os.ReadFile(filepath.Join(e.Config.StateDir, "history.jsonl")) //nolint:gosec // state root is operator-configured and private
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var operations []Operation
	for number, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var operation Operation
		if err := json.Unmarshal([]byte(line), &operation); err != nil {
			return nil, fmt.Errorf("backup: corrupt history line %d: %w", number+1, err)
		}
		operations = append(operations, operation)
	}
	return operations, nil
}
