package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTransactionCommitsValidatedConfiguration(t *testing.T) {
	target := filepath.Join(t.TempDir(), "service.conf")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	validated := false
	reloaded := false
	transaction := NewTransaction(target,
		func(_ context.Context, prepared string) error {
			content, err := os.ReadFile(prepared)
			validated = err == nil && string(content) == "new"
			return err
		},
		func(context.Context) error { reloaded = true; return nil },
	)
	if err := transaction.Execute(context.Background(), []byte("new")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "new" || !validated || !reloaded {
		t.Fatalf("transaction was not completed: %q, %v", content, err)
	}
}

func TestTransactionRollsBackWhenReloadFails(t *testing.T) {
	target := filepath.Join(t.TempDir(), "service.conf")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	reloads := 0
	transaction := NewTransaction(target, nil, func(context.Context) error {
		reloads++
		if reloads == 1 {
			return errors.New("reload rejected")
		}
		return nil
	})
	if err := transaction.Execute(context.Background(), []byte("bad")); err == nil {
		t.Fatal("expected reload failure")
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "old" {
		t.Fatalf("original was not restored: %q, %v", content, err)
	}
	if reloads != 2 {
		t.Fatalf("reload calls = %d, want 2", reloads)
	}
}

func TestTransactionRejectsSymlinkTarget(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("symlinks require additional privileges on Windows")
	}
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(real, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := NewTransaction(link, nil, nil).Execute(context.Background(), []byte("new")); err == nil {
		t.Fatal("expected symlink target to be rejected")
	}
}
