package module

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type recoveryJobStore struct {
	jobs    map[JobState][]Job
	updated map[string]JobState
}

func (s *recoveryJobStore) Create(context.Context, *Job) error { return nil }
func (s *recoveryJobStore) FindByID(context.Context, string) (*Job, error) {
	return nil, errors.New("not implemented")
}
func (s *recoveryJobStore) UpdateState(_ context.Context, id string, state JobState, _, _ string) error {
	s.updated[id] = state
	return nil
}
func (s *recoveryJobStore) AppendOutput(context.Context, string, string) error { return nil }
func (s *recoveryJobStore) ListByState(_ context.Context, state JobState) ([]Job, error) {
	return s.jobs[state], nil
}
func (s *recoveryJobStore) List(context.Context, JobFilter) (*JobPage, error) {
	return nil, errors.New("not implemented")
}
func (s *recoveryJobStore) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

func TestRecoverInterruptedJobsMarksPersistedWorkTerminal(t *testing.T) {
	store := &recoveryJobStore{
		jobs: map[JobState][]Job{
			JobPending: {{ID: "pending"}},
			JobRunning: {{ID: "running"}},
		},
		updated: make(map[string]JobState),
	}
	service := &Service{jobs: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := service.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pending", "running"} {
		if store.updated[id] != JobError {
			t.Fatalf("job %s state = %q, want error", id, store.updated[id])
		}
	}
}
