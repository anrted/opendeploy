package module

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

func (s *Service) GetJob(ctx context.Context, jobID string) (*Job, error) {
	return s.jobs.FindByID(ctx, jobID)
}

func (s *Service) ListJobs(ctx context.Context, filter JobFilter) (*JobPage, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.jobs.List(ctx, filter)
}

func (s *Service) CancelJob(ctx context.Context, jobID string) error {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.State != JobPending && job.State != JobRunning {
		return apperrors.New(409, apperrors.CodeConflict, "only pending or running tasks can be canceled")
	}
	s.cancelMu.Lock()
	cancel := s.cancels[jobID]
	s.cancelMu.Unlock()
	if cancel == nil {
		return apperrors.New(409, apperrors.CodeConflict, "task is no longer attached to this process")
	}
	cancel()
	return s.jobs.UpdateState(ctx, jobID, JobCanceled, job.Output, "canceled by user")
}

func (s *Service) DeleteJob(ctx context.Context, jobID string) error {
	if _, err := s.jobs.FindByID(ctx, jobID); err != nil {
		return err
	}
	return s.jobs.Delete(ctx, jobID)
}

func (s *Service) RetryJob(ctx context.Context, jobID string) (string, error) {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return "", err
	}
	if job.State != JobError && job.State != JobCanceled {
		return "", apperrors.New(409, apperrors.CodeConflict, "only failed or canceled tasks can be retried")
	}
	var payload struct {
		ModuleID string `json:"module_id"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil || payload.ModuleID == "" {
		return "", apperrors.InvalidInput("task payload cannot be retried")
	}
	current := s.registry.Find(payload.ModuleID)
	if current == nil {
		return "", apperrors.New(404, apperrors.CodeModuleNotFound, "task module is no longer available")
	}
	switch job.Type {
	case JobInstall:
		return s.startJob(ctx, JobInstall, payload.ModuleID, current.Install, func(doneCtx context.Context) {
			now := nowUTC()
			_ = s.repo.Upsert(doneCtx, &Record{ID: payload.ModuleID, Name: current.Name(), State: StateInstalled, InstalledAt: &now, UpdatedAt: now})
		})
	case JobUninstall:
		return s.startJob(ctx, JobUninstall, payload.ModuleID, current.Uninstall, func(doneCtx context.Context) {
			_ = s.repo.UpdateState(doneCtx, payload.ModuleID, StateAvailable)
		})
	default:
		return "", apperrors.InvalidInput("task type does not support retry")
	}
}

func (s *Service) RecoverInterruptedJobs(ctx context.Context) error {
	for _, state := range []JobState{JobPending, JobRunning} {
		jobs, err := s.jobs.ListByState(ctx, state)
		if err != nil {
			return fmt.Errorf("list interrupted %s jobs: %w", state, err)
		}
		for _, job := range jobs {
			message := "operation interrupted by Core restart; runtime state will be reconciled"
			if err := s.jobs.UpdateState(ctx, job.ID, JobError, job.Output, message); err != nil {
				return fmt.Errorf("mark interrupted job %s: %w", job.ID, err)
			}
			s.logger.WarnContext(ctx, "module job recovered as interrupted", "job_id", job.ID, "previous_state", state)
		}
	}
	return nil
}
