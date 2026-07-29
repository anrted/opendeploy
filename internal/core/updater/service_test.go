package updater

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	secureupdate "github.com/anrted/opendeploy/internal/update"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"v0.2.0", "v0.1.0", 1},
		{"v0.1.0", "v0.1.0-alpha", 1},
		{"v0.1.0-alpha", "v0.1.0-alpha-2-gabcdef", -1},
		{"v1.0.0", "dev", 0},
	}
	for _, test := range tests {
		got := compareVersions(test.left, test.right)
		if got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

type recordingAgent struct {
	path string
	mode uint32
	data []byte
}

func (a *recordingAgent) FileRead(_ context.Context, _ string) ([]byte, error) {
	return a.data, nil
}

func (a *recordingAgent) FileWrite(_ context.Context, path string, data []byte, mode uint32) error {
	a.path = path
	a.mode = mode
	a.data = append([]byte(nil), data...)
	return nil
}

func TestApplyCreatesRestrictedUpdateRequest(t *testing.T) {
	agent := &recordingAgent{}
	service := NewService("v0.1.0", "old", agent)
	service.cached = &Status{UpdateAvailable: true, LatestVersion: "v0.2.0"}
	service.cachedAt = time.Now()
	if err := service.Apply(context.Background(), "release"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if agent.path != updateRequest {
		t.Fatalf("path = %q, want %q", agent.path, updateRequest)
	}
	if agent.mode != 0o600 {
		t.Fatalf("mode = %#o, want 0600", agent.mode)
	}
	var request struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Tag       string `json:"tag"`
	}
	if err := json.Unmarshal(agent.data, &request); err != nil {
		t.Fatalf("request JSON: %v", err)
	}
	if request.Schema != "opendeploy.update-request/v1" || request.Operation != "apply" || request.Tag != "v0.2.0" {
		t.Fatalf("request = %#v", request)
	}
}

func TestApplyRejectsDevelopmentChannel(t *testing.T) {
	service := NewService("v0.1.0", "old", &recordingAgent{})
	if err := service.Apply(context.Background(), "dev"); err == nil {
		t.Fatal("development update channel was accepted")
	}
}

func TestApplyReportsQueuedOperation(t *testing.T) {
	agent := &recordingAgent{data: []byte("{\"operation\":\"apply\"}\n")}
	service := NewService("v0.1.0", "old", agent)
	service.cached = &Status{UpdateAvailable: true, LatestVersion: "v0.2.0"}
	service.cachedAt = time.Now()

	err := service.Apply(context.Background(), "stable")
	if !errors.Is(err, ErrOperationQueued) {
		t.Fatalf("Apply error = %v, want ErrOperationQueued", err)
	}
}

func TestRollbackCreatesRestrictedTypedRequest(t *testing.T) {
	agent := &recordingAgent{}
	service := NewService("v0.2.0", "commit", agent)
	if err := service.Rollback(context.Background(), "transaction-1"); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	var request struct {
		Operation     string `json:"operation"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(agent.data, &request); err != nil {
		t.Fatal(err)
	}
	if agent.mode != 0o600 || request.Operation != "rollback" || request.TransactionID != "transaction-1" {
		t.Fatalf("request = %#v, mode = %#o", request, agent.mode)
	}
}

func TestHistoryReadsJSONLJournal(t *testing.T) {
	agent := &recordingAgent{data: []byte(
		"{\"id\":\"one\",\"to_version\":\"v0.2.0\",\"commit\":\"0123456789abcdef0123456789abcdef01234567\",\"status\":\"succeeded\",\"started_at\":\"2026-07-29T00:00:00Z\",\"completed_at\":\"2026-07-29T00:01:00Z\"}\n",
	)}
	service := NewService("v0.2.0", "commit", agent)
	entries, err := service.History(context.Background())
	if err != nil || len(entries) != 1 || entries[0].ID != "one" {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
}

func TestBackupAndRestoreCreateRestrictedTypedRequests(t *testing.T) {
	agent := &recordingAgent{}
	service := NewService("v0.2.0", "commit", agent)
	if err := service.CreateBackup(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	var request secureupdate.UpdateRequest
	if err := json.Unmarshal(agent.data, &request); err != nil {
		t.Fatal(err)
	}
	if request.Operation != "backup" || request.Reason != "manual" || agent.mode != 0o600 {
		t.Fatalf("backup request = %#v, mode = %#o", request, agent.mode)
	}
	if err := service.RestoreBackup(context.Background(), "../unsafe.tar.gz"); err == nil {
		t.Fatal("unsafe restore archive accepted")
	}
	agent.data = nil // simulate privileged worker consuming the create request
	if err := service.RestoreBackup(context.Background(), "opendeploy-20260729T010203Z.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(agent.data, &request); err != nil {
		t.Fatal(err)
	}
	if request.Operation != "restore" || request.Archive != "opendeploy-20260729T010203Z.tar.gz" {
		t.Fatalf("restore request = %#v", request)
	}
}
