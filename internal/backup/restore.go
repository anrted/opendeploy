package backup

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
	"os"
	"path/filepath"
	"strings"
)

func (e *Engine) Verify(ctx context.Context, archivePath string) (manifest Manifest, err error) {
	if err := e.prepare(); err != nil {
		return manifest, err
	}
	started := e.Now().UTC()
	operation := Operation{
		ID:   "verify-" + started.Format("20060102T150405.000000000Z"),
		Type: "verify", Archive: archivePath, Status: "running", StartedAt: started,
	}
	defer func() {
		operation.FinishedAt, operation.BackupID = e.Now().UTC(), manifest.ID
		if err != nil {
			operation.Status, operation.Error = "failed", err.Error()
		} else {
			operation.Status = "succeeded"
		}
		_ = e.appendHistory(operation)
	}()
	manifest, _, err = e.extractVerified(ctx, archivePath, "")
	return manifest, err
}

func (e *Engine) Restore(ctx context.Context, archivePath string) (manifest Manifest, err error) { //nolint:gocyclo
	if err := e.prepare(); err != nil {
		return manifest, err
	}
	started := e.Now().UTC()
	operation := Operation{
		ID:   "restore-" + started.Format("20060102T150405.000000000Z"),
		Type: "restore", Archive: archivePath, Status: "running", StartedAt: started,
	}
	defer func() {
		operation.FinishedAt = e.Now().UTC()
		operation.BackupID = manifest.ID
		if err != nil {
			operation.Status, operation.Error = "failed", err.Error()
		} else {
			operation.Status = "succeeded"
		}
		_ = e.appendHistory(operation)
	}()
	stage, err := os.MkdirTemp(e.Config.StateDir, "restore-*")
	if err != nil {
		return manifest, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	manifest, extracted, err := e.extractVerified(ctx, archivePath, stage)
	if err != nil {
		return manifest, err
	}
	sourceMap := make(map[string]Source, len(e.Config.Sources))
	for _, source := range e.Config.Sources {
		sourceMap[source.ID] = source
	}
	for _, entry := range manifest.Entries {
		if _, ok := sourceMap[entry.SourceID]; !ok {
			return manifest, fmt.Errorf("backup: target mapping for source %q is missing", entry.SourceID)
		}
	}
	if e.Runtime != nil {
		if err := e.Runtime.BeforeRestore(ctx); err != nil {
			return manifest, fmt.Errorf("backup: prepare services for restore: %w", err)
		}
	}
	runtimePrepared := e.Runtime != nil
	defer func() {
		if err != nil && runtimePrepared {
			err = errors.Join(err, e.Runtime.AfterRestore(context.WithoutCancel(ctx)))
		}
	}()
	type restoreTarget struct {
		path        string
		snapshot    string
		hadOriginal bool
		mode        os.FileMode
	}
	var changed []restoreTarget
	rollback := func() error {
		var rollbackErr error
		for index := len(changed) - 1; index >= 0; index-- {
			item := changed[index]
			if item.hadOriginal {
				rollbackErr = errors.Join(rollbackErr, atomicCopy(item.snapshot, item.path, item.mode))
			} else {
				rollbackErr = errors.Join(rollbackErr, removeRegular(item.path))
			}
		}
		return rollbackErr
	}
	for _, entry := range manifest.Entries {
		source, ok := sourceMap[entry.SourceID]
		if !ok {
			return manifest, fmt.Errorf("backup: target mapping for source %q is missing", entry.SourceID)
		}
		target := source.Path
		if !source.Database && !source.File {
			target = filepath.Join(source.Path, filepath.FromSlash(entry.Path))
		}
		item := restoreTarget{path: target}
		if info, statErr := os.Lstat(target); statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				info, statErr = os.Stat(target)
			}
			if statErr != nil || !info.Mode().IsRegular() {
				return manifest, fmt.Errorf("backup: restore target %q is not a regular file", target)
			}
			item.hadOriginal = true
			item.mode = info.Mode()
			item.snapshot = filepath.Join(stage, "rollback", fmt.Sprintf("%d", len(changed)))
			if err := atomicCopy(target, item.snapshot, info.Mode()); err != nil {
				return manifest, fmt.Errorf("backup: snapshot current %s: %w", target, err)
			}
		} else if !os.IsNotExist(statErr) {
			return manifest, statErr
		}
		changed = append(changed, item)
		if err := atomicCopy(extracted[entry.SourceID+"/"+entry.Path], target, os.FileMode(entry.Mode)); err != nil {
			rollbackErr := rollback()
			return manifest, fmt.Errorf("backup: restore %s/%s; transaction rolled back: %w",
				entry.SourceID, entry.Path, errors.Join(err, rollbackErr))
		}
	}
	if e.Runtime != nil {
		if err := e.Runtime.AfterRestore(ctx); err != nil {
			rollbackErr := rollback()
			recoveryErr := e.Runtime.AfterRestore(context.WithoutCancel(ctx))
			runtimePrepared = false
			return manifest, fmt.Errorf("backup: post-restore health gate failed; files rolled back: %w",
				errors.Join(err, rollbackErr, recoveryErr))
		}
		runtimePrepared = false
	}
	return manifest, nil
}

func (e *Engine) extractVerified(ctx context.Context, archivePath, destination string) (Manifest, map[string]string, error) { //nolint:gocyclo
	if err := e.Config.validate(); err != nil {
		return Manifest{}, nil, err
	}
	input, err := os.Open(archivePath) //nolint:gosec // path is explicitly supplied by the root operator
	if err != nil {
		return Manifest{}, nil, err
	}
	defer func() { _ = input.Close() }()
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("backup: open gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()
	reader := tar.NewReader(gzipReader)
	header, err := reader.Next()
	if err != nil || header.Name != manifestName || header.Typeflag != tar.TypeReg || header.Size > 16<<20 {
		return Manifest{}, nil, fmt.Errorf("backup: manifest must be the first regular entry")
	}
	var manifest Manifest
	decoder := json.NewDecoder(io.LimitReader(reader, header.Size))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, nil, fmt.Errorf("backup: parse manifest: %w", err)
	}
	if manifest.Schema != Schema || manifest.ID == "" || manifest.CreatedAt.IsZero() ||
		len(manifest.Entries) > e.Config.MaxEntries {
		return manifest, nil, fmt.Errorf("backup: unsupported or invalid manifest")
	}
	expected := make(map[string]Entry, len(manifest.Entries))
	presentSources := make(map[string]bool, len(e.Config.Sources))
	var declaredBytes int64
	for _, entry := range manifest.Entries {
		key := entry.SourceID + "/" + entry.Path
		if _, duplicate := expected[key]; duplicate || !safeSourceID(entry.SourceID) ||
			!safeRelative(entry.Path) || len(entry.SHA256) != sha256.Size*2 ||
			entry.Size < 0 || entry.Size > e.Config.MaxBytes-declaredBytes || entry.Mode > 0o777 {
			return manifest, nil, fmt.Errorf("backup: invalid manifest entry %q", key)
		}
		expected[key] = entry
		declaredBytes += entry.Size
		presentSources[entry.SourceID] = true
	}
	if declaredBytes != manifest.TotalBytes || declaredBytes > e.Config.MaxBytes {
		return manifest, nil, fmt.Errorf("backup: declared size is invalid")
	}
	for _, source := range e.Config.Sources {
		if source.Required && !presentSources[source.ID] {
			return manifest, nil, fmt.Errorf("backup: required source %q is missing", source.ID)
		}
	}
	extracted := make(map[string]string, len(expected))
	for {
		select {
		case <-ctx.Done():
			return manifest, nil, ctx.Err()
		default:
		}
		header, err = reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return manifest, nil, fmt.Errorf("backup: read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || !strings.HasPrefix(header.Name, "data/") {
			return manifest, nil, fmt.Errorf("backup: forbidden archive entry %q", header.Name)
		}
		key := strings.TrimPrefix(header.Name, "data/")
		entry, ok := expected[key]
		if !ok || header.Size != entry.Size {
			return manifest, nil, fmt.Errorf("backup: unexpected entry %q", key)
		}
		hash := sha256.New()
		var output *os.File
		target := ""
		if destination != "" {
			target = filepath.Join(destination, filepath.FromSlash(key))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return manifest, nil, err
			}
			output, err = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // target is confined below private staging
			if err != nil {
				return manifest, nil, err
			}
		}
		writer := io.Writer(hash)
		if output != nil {
			writer = io.MultiWriter(hash, output)
		}
		written, copyErr := io.CopyN(writer, reader, entry.Size)
		closeErr := error(nil)
		if output != nil {
			closeErr = output.Close()
		}
		if copyErr != nil || closeErr != nil || written != entry.Size ||
			hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return manifest, nil, fmt.Errorf("backup: integrity check failed for %q: %w", key, errors.Join(copyErr, closeErr))
		}
		delete(expected, key)
		extracted[key] = target
	}
	if len(expected) != 0 {
		return manifest, nil, fmt.Errorf("backup: archive is incomplete")
	}
	return manifest, extracted, nil
}

func safeRelative(path string) bool {
	if path == "" || strings.Contains(path, "\\") || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != ".." && !strings.HasPrefix(clean, "../")
}

func atomicCopy(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return err
	}
	input, err := os.Open(source) //nolint:gosec // source is confined to verified private staging
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".restore-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err := io.Copy(temp, input); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(mode.Perm()); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, destination)
}

func removeRegular(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup: refusing to remove non-regular rollback target %q", path)
	}
	return os.Remove(path)
}
