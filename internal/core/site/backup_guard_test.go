package site

import (
	"context"
	"errors"
	"testing"
)

type rejectingSiteBackupGuard struct{ calls int }

func (g *rejectingSiteBackupGuard) CreateBackupAndWait(context.Context, string) error {
	g.calls++
	return errors.New("backup failed")
}

func TestCriticalSiteMutationFailsClosedWhenBackupFails(t *testing.T) {
	service := &Service{}
	guard := &rejectingSiteBackupGuard{}
	service.SetBackupGuard(guard)
	if err := service.backupCritical(context.Background(), "update", "site-id"); err == nil {
		t.Fatal("critical site mutation accepted backup failure")
	}
	if guard.calls != 1 {
		t.Fatalf("backup calls = %d, want 1", guard.calls)
	}
}
