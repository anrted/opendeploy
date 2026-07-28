package module

import (
	"context"
	"errors"
	"testing"
)

type rejectingBackupGuard struct{ calls int }

func (g *rejectingBackupGuard) CreateBackupAndWait(context.Context, string) error {
	g.calls++
	return errors.New("backup failed")
}

func TestCriticalModuleMutationFailsClosedWhenBackupFails(t *testing.T) {
	service := &Service{}
	guard := &rejectingBackupGuard{}
	service.SetBackupGuard(guard)
	if err := service.backupCritical(context.Background(), "install", "nginx"); err == nil {
		t.Fatal("critical module mutation accepted backup failure")
	}
	if guard.calls != 1 {
		t.Fatalf("backup calls = %d, want 1", guard.calls)
	}
}
