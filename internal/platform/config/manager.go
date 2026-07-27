package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const configCommandTimeout = 30 * time.Second

// Transaction applies one configuration file using prepare, validate, commit,
// reload and rollback steps. Temporary and backup files are unique and created
// beside the target so the final rename stays on the same filesystem.
type Transaction struct {
	TargetFile string
	Validator  func(ctx context.Context, tempFile string) error
	Reloader   func(ctx context.Context) error
	Mode       os.FileMode
}

func NewTransaction(targetFile string, validator func(context.Context, string) error, reloader func(context.Context) error) *Transaction {
	return &Transaction{TargetFile: targetFile, Validator: validator, Reloader: reloader, Mode: 0o640}
}

func (t *Transaction) Execute(ctx context.Context, content []byte) error {
	if !filepath.IsAbs(t.TargetFile) {
		return fmt.Errorf("config transaction: target must be absolute")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory := filepath.Dir(filepath.Clean(t.TargetFile))
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("config transaction: create target directory: %w", err)
	}
	if info, err := os.Lstat(t.TargetFile); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config transaction: symlink targets are forbidden")
	}

	prepared, err := os.CreateTemp(directory, "."+filepath.Base(t.TargetFile)+".prepare-*")
	if err != nil {
		return fmt.Errorf("config transaction: create prepared file: %w", err)
	}
	preparedPath := prepared.Name()
	defer os.Remove(preparedPath)
	mode := t.Mode.Perm()
	if mode == 0 {
		mode = 0o640
	}
	if err := prepared.Chmod(mode); err != nil {
		_ = prepared.Close()
		return fmt.Errorf("config transaction: chmod prepared file: %w", err)
	}
	if _, err := prepared.Write(content); err != nil {
		_ = prepared.Close()
		return fmt.Errorf("config transaction: write prepared file: %w", err)
	}
	if err := prepared.Sync(); err != nil {
		_ = prepared.Close()
		return fmt.Errorf("config transaction: sync prepared file: %w", err)
	}
	if err := prepared.Close(); err != nil {
		return fmt.Errorf("config transaction: close prepared file: %w", err)
	}
	if t.Validator != nil {
		if err := t.Validator(ctx, preparedPath); err != nil {
			return fmt.Errorf("config transaction: validation failed: %w", err)
		}
	}

	backupPath, hadOriginal, err := backupFile(t.TargetFile, directory)
	if err != nil {
		return err
	}
	if backupPath != "" {
		defer os.Remove(backupPath)
	}
	if err := os.Rename(preparedPath, t.TargetFile); err != nil {
		return fmt.Errorf("config transaction: commit: %w", err)
	}
	if err := syncDir(directory); err != nil {
		rollbackErr := restoreBackup(t.TargetFile, backupPath, hadOriginal)
		return errors.Join(fmt.Errorf("config transaction: sync commit: %w", err), rollbackErr)
	}
	if t.Reloader == nil {
		return nil
	}
	if err := t.Reloader(ctx); err != nil {
		rollbackErr := restoreBackup(t.TargetFile, backupPath, hadOriginal)
		rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), configCommandTimeout)
		defer rollbackCancel()
		reloadRollbackErr := t.Reloader(rollbackCtx)
		return errors.Join(
			fmt.Errorf("config transaction: reload failed: %w", err),
			rollbackErr,
			wrapIfError("config transaction: rollback reload", reloadRollbackErr),
		)
	}
	return nil
}

func backupFile(target, directory string) (string, bool, error) {
	source, err := os.Open(target)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("config transaction: open original: %w", err)
	}
	defer source.Close()
	backup, err := os.CreateTemp(directory, "."+filepath.Base(target)+".backup-*")
	if err != nil {
		return "", false, fmt.Errorf("config transaction: create backup: %w", err)
	}
	path := backup.Name()
	if _, err := io.Copy(backup, source); err != nil {
		_ = backup.Close()
		_ = os.Remove(path)
		return "", false, fmt.Errorf("config transaction: copy backup: %w", err)
	}
	if err := backup.Sync(); err != nil {
		_ = backup.Close()
		_ = os.Remove(path)
		return "", false, fmt.Errorf("config transaction: sync backup: %w", err)
	}
	if err := backup.Close(); err != nil {
		_ = os.Remove(path)
		return "", false, fmt.Errorf("config transaction: close backup: %w", err)
	}
	return path, true, nil
}

func restoreBackup(target, backup string, hadOriginal bool) error {
	if !hadOriginal {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("config transaction: remove failed commit: %w", err)
		}
		return syncDir(filepath.Dir(target))
	}
	if err := os.Rename(backup, target); err != nil {
		return fmt.Errorf("config transaction: restore backup: %w", err)
	}
	return syncDir(filepath.Dir(target))
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func wrapIfError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// CmdValidator runs a fixed executable without a shell, under a deadline and
// with bounded diagnostic output.
func CmdValidator(name string, args ...string) func(context.Context, string) error {
	return func(ctx context.Context, tempFile string) error {
		if name == "" || strings.ContainsAny(name, `/\`) {
			return fmt.Errorf("validator executable must be a basename")
		}
		commandArgs := append([]string(nil), args...)
		for i, arg := range commandArgs {
			if arg == "{temp}" {
				commandArgs[i] = tempFile
			}
		}
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, configCommandTimeout)
			defer cancel()
		}
		command := exec.CommandContext(ctx, name, commandArgs...)
		command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
		var output boundedOutput
		command.Stdout, command.Stderr = &output, &output
		if err := command.Run(); err != nil {
			return fmt.Errorf("validator %s failed: %w: %s", name, err, output.String())
		}
		return nil
	}
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(value []byte) (int, error) {
	original := len(value)
	if b.Len() >= 1<<20 {
		return original, nil
	}
	remaining := 1<<20 - b.Len()
	if len(value) > remaining {
		value = value[:remaining]
	}
	_, _ = b.Buffer.Write(value)
	return original, nil
}
