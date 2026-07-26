package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Transaction defines steps for a configuration change.
type Transaction struct {
	TargetFile string
	TempFile   string
	BackupFile string
	Validator  func(ctx context.Context, tempFile string) error
	Reloader   func(ctx context.Context) error
}

func NewTransaction(targetFile string, validator func(ctx context.Context, tempFile string) error, reloader func(ctx context.Context) error) *Transaction {
	return &Transaction{
		TargetFile: targetFile,
		TempFile:   targetFile + ".tmp",
		BackupFile: targetFile + ".bak",
		Validator:  validator,
		Reloader:   reloader,
	}
}

// Execute performs the transaction.
func (t *Transaction) Execute(ctx context.Context, content []byte) error {
	// 1. Write to temp file
	if err := os.WriteFile(t.TempFile, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(t.TempFile)

	// 2. Validate
	if t.Validator != nil {
		if err := t.Validator(ctx, t.TempFile); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// 3. Backup original (if exists)
	if _, err := os.Stat(t.TargetFile); err == nil {
		if err := os.Rename(t.TargetFile, t.BackupFile); err != nil {
			return fmt.Errorf("failed to backup: %w", err)
		}
	}

	// 4. Atomic Replace (move temp to target)
	// Rename is atomic on POSIX systems if they are on the same mount
	if err := os.Rename(t.TempFile, t.TargetFile); err != nil {
		// Rollback attempt
		_ = os.Rename(t.BackupFile, t.TargetFile)
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// 5. Reload Service
	if t.Reloader != nil {
		if err := t.Reloader(ctx); err != nil {
			// Rollback
			_ = os.Rename(t.BackupFile, t.TargetFile)
			// Trigger another reload since we reverted
			_ = t.Reloader(ctx)
			return fmt.Errorf("failed to reload service: %w", err)
		}
	}

	return nil
}

// CmdValidator returns a validator function that runs a command.
func CmdValidator(name string, args ...string) func(context.Context, string) error {
	return func(ctx context.Context, tempFile string) error {
		cmdArgs := make([]string, len(args))
		copy(cmdArgs, args)
		// replace placeholder {temp} with actual temp file if needed, or assume it's just validated normally
		for i, arg := range cmdArgs {
			if arg == "{temp}" {
				cmdArgs[i] = tempFile
			}
		}
		cmd := exec.CommandContext(ctx, name, cmdArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("validator %s failed: %w, output: %s", name, err, string(out))
		}
		return nil
	}
}
