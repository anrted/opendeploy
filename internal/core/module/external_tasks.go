package module

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// StartTask lets built-in modules publish background work through the same
// persistent Task Manager used by lifecycle operations.
func (s *Service) StartTask(ctx context.Context, name, taskType string, payload map[string]string, work func(context.Context) (string, error)) (string, error) {
	encoded, _ := json.Marshal(payload)
	job := &Job{
		ID: uuid.NewString(), Name: name, Type: JobType(taskType), Payload: string(encoded),
		State: JobPending, CreatedAt: nowUTC(),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return "", err
	}
	background, cancel := context.WithTimeout(context.Background(), moduleJobTimeout)
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
		_ = s.jobs.UpdateState(background, job.ID, JobRunning, "", "")
		output, err := work(background)
		persist, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer persistCancel()
		if err != nil {
			state := JobError
			if errors.Is(err, context.Canceled) {
				state = JobCanceled
			}
			_ = s.jobs.UpdateState(persist, job.ID, state, output, err.Error())
			return
		}
		_ = s.jobs.UpdateState(persist, job.ID, JobSuccess, output, "")
	}()
	return job.ID, nil
}
