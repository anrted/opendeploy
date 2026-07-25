package updater

import (
	"context"
	"testing"
	"time"
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
}

func (a *recordingAgent) FileWrite(_ context.Context, path string, _ []byte, mode uint32) error {
	a.path = path
	a.mode = mode
	return nil
}

func TestApplyCreatesRestrictedUpdateRequest(t *testing.T) {
	agent := &recordingAgent{}
	service := NewService("v0.1.0", "old", agent)
	service.cached = &Status{UpdateAvailable: true}
	service.cachedAt = time.Now()

	if err := service.Apply(context.Background(), "dev"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if agent.path != updateRequest {
		t.Fatalf("path = %q, want %q", agent.path, updateRequest)
	}
	if agent.mode != 0o600 {
		t.Fatalf("mode = %#o, want 0600", agent.mode)
	}
}
