package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerWritesManagedCronAtomically(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerWithPaths(filepath.Join(root, "state", "jobs.json"), filepath.Join(root, "cron.d", "opendeploy"))
	job := Job{ID: "backup", Name: "Backup", Command: "/usr/bin/true", User: currentUser(t), Expression: "0 3 * * *", Enabled: true}
	if _, err := manager.Create(job); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "cron.d", "opendeploy")) // #nosec G304 -- test temp directory
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "0 3 * * * root /usr/bin/opendeploy-agent --cron-run=backup # opendeploy:backup") {
		t.Fatalf("unexpected cron file: %s", content)
	}
	if _, err := manager.SetEnabled("backup", false); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(filepath.Join(root, "cron.d", "opendeploy")) // #nosec G304 -- test temp directory
	if strings.Contains(string(content), "opendeploy:backup") {
		t.Fatalf("disabled job remained in cron file: %s", content)
	}
}

func TestValidationRejectsDangerousCommandsAndInvalidSchedules(t *testing.T) {
	base := Job{ID: "safe", Name: "Safe", Command: "/usr/bin/true", User: currentUser(t), Expression: "0 3 * * *"}
	if _, err := ValidateJob(base); err != nil {
		t.Fatalf("valid job rejected: %v", err)
	}
	base.Command = "rm -rf /"
	if _, err := ValidateJob(base); err == nil {
		t.Fatal("dangerous command accepted")
	}
	base.Command, base.Expression = "/usr/bin/true", "90 * * * *"
	if _, err := ValidateJob(base); err == nil {
		t.Fatal("invalid expression accepted")
	}
}

func TestManagerCRUD(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerWithPaths(filepath.Join(root, "jobs.json"), filepath.Join(root, "opendeploy"))
	job := Job{ID: "cleanup", Name: "Cleanup", Command: "/usr/bin/true", User: currentUser(t), Expression: "@daily"}
	if _, err := manager.Create(job); err == nil {
		t.Fatal("non-five-field expression accepted")
	}
	job.Expression = "0 1 * * *"
	created, err := manager.Create(job)
	if err != nil {
		t.Fatal(err)
	}
	created.Description = "updated"
	if _, err := manager.Update(created); err != nil {
		t.Fatal(err)
	}
	got, err := manager.Get(job.ID)
	if err != nil || got.Description != "updated" {
		t.Fatalf("unexpected job: %#v, %v", got, err)
	}
	if err := manager.Delete(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Get(job.ID); !os.IsNotExist(err) {
		t.Fatalf("deleted job lookup error = %v", err)
	}
}

func currentUser(t *testing.T) string {
	t.Helper()
	value := os.Getenv("USERNAME")
	if value == "" {
		value = os.Getenv("USER")
	}
	if value == "" {
		t.Skip("current user is unavailable")
	}
	return value
}
