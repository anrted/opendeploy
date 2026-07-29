package module

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/google/uuid"
)

// startJob is kept behind the Service facade, while this file isolates job
// persistence, cancellation and detached execution from module lifecycle logic.
func (s *Service) startJob(
	ctx context.Context,
	jobType JobType,
	moduleID string,
	work func(ctx context.Context) error,
	onSuccess func(ctx context.Context),
) (string, error) {
	payload, _ := json.Marshal(map[string]string{"module_id": moduleID})
	job := &Job{
		ID: uuid.New().String(), Name: fmt.Sprintf("%s %s", jobType, moduleID),
		Type: jobType, Payload: string(payload), State: JobPending,
		CreatedAt: nowUTC(), Progress: 0,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return "", apperrors.Internal("create job", err)
	}

	serverID := servercontext.ID(ctx)
	bgCtx, cancel := context.WithTimeout(servercontext.WithID(context.Background(), serverID), moduleJobTimeout)
	s.cancelMu.Lock()
	s.cancels[job.ID] = cancel
	s.cancelMu.Unlock()
	go func() {
		defer func() {
			cancel()
			s.cancelMu.Lock()
			delete(s.cancels, job.ID)
			s.cancelMu.Unlock()
		}()
		_ = s.jobs.UpdateState(bgCtx, job.ID, JobRunning, "", "")

		if err := work(bgCtx); err != nil {
			s.logger.Error("module job failed", "job_id", job.ID, "type", jobType, "error", err)
			persistCtx, persistCancel := context.WithTimeout(servercontext.WithID(context.Background(), serverID), 5*time.Second)
			defer persistCancel()
			state := JobError
			if errors.Is(err, context.Canceled) {
				state = JobCanceled
			}
			_ = s.jobs.UpdateState(persistCtx, job.ID, state, "", err.Error())
			s.publishEvent(persistCtx, "job.error", map[string]string{"job_id": job.ID})
			return
		}
		if bgCtx.Err() != nil {
			persistCtx, persistCancel := context.WithTimeout(servercontext.WithID(context.Background(), serverID), 5*time.Second)
			defer persistCancel()
			_ = s.jobs.UpdateState(persistCtx, job.ID, JobCanceled, "", bgCtx.Err().Error())
			s.publishEvent(persistCtx, "job.canceled", map[string]string{"job_id": job.ID})
			return
		}

		persistCtx, persistCancel := context.WithTimeout(servercontext.WithID(context.Background(), serverID), 5*time.Second)
		defer persistCancel()
		onSuccess(persistCtx)
		_ = s.jobs.UpdateState(persistCtx, job.ID, JobSuccess, "", "")
		s.publishEvent(persistCtx, "job.done", map[string]string{"job_id": job.ID})
	}()
	return job.ID, nil
}
