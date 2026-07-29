package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	agentCron "github.com/anrted/opendeploy/internal/agent/cron"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) CronList(context.Context, *agentv1.CronListRequest) (*agentv1.CronListResponse, error) {
	jobs, err := s.cron.List()
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.CronListResponse{}
	for _, job := range jobs {
		response.Jobs = append(response.Jobs, cronJobToProto(job))
	}
	return response, nil
}

func (s *Service) CronGet(_ context.Context, req *agentv1.CronGetRequest) (*agentv1.CronJob, error) {
	job, err := s.cron.Get(req.GetId())
	if err != nil {
		return nil, cronError(err)
	}
	return cronJobToProto(job), nil
}

func (s *Service) CronCreate(_ context.Context, req *agentv1.CronMutationRequest) (*agentv1.CronJob, error) {
	job, err := s.cron.Create(cronJobFromProto(req.GetJob()))
	if err != nil {
		return nil, cronError(err)
	}
	return cronJobToProto(job), nil
}

func (s *Service) CronUpdate(_ context.Context, req *agentv1.CronMutationRequest) (*agentv1.CronJob, error) {
	job, err := s.cron.Update(cronJobFromProto(req.GetJob()))
	if err != nil {
		return nil, cronError(err)
	}
	return cronJobToProto(job), nil
}

func (s *Service) CronDelete(_ context.Context, req *agentv1.CronIDRequest) (*agentv1.CronOperationResponse, error) {
	if err := s.cron.Delete(req.GetId()); err != nil {
		return nil, cronError(err)
	}
	return &agentv1.CronOperationResponse{Success: true}, nil
}

func (s *Service) CronEnable(_ context.Context, req *agentv1.CronIDRequest) (*agentv1.CronJob, error) {
	job, err := s.cron.SetEnabled(req.GetId(), true)
	if err != nil {
		return nil, cronError(err)
	}
	return cronJobToProto(job), nil
}

func (s *Service) CronDisable(_ context.Context, req *agentv1.CronIDRequest) (*agentv1.CronJob, error) {
	job, err := s.cron.SetEnabled(req.GetId(), false)
	if err != nil {
		return nil, cronError(err)
	}
	return cronJobToProto(job), nil
}

func (s *Service) CronRun(ctx context.Context, req *agentv1.CronRunRequest) (*agentv1.CronRunRecord, error) {
	run, err := s.cron.Run(ctx, req.GetId(), req.GetTrigger(), req.GetActor())
	if err != nil && run.ID == "" {
		return nil, cronError(err)
	}
	return cronRunToProto(run), nil
}

func (s *Service) CronHistory(_ context.Context, req *agentv1.CronHistoryRequest) (*agentv1.CronHistoryResponse, error) {
	return s.cronHistory(req.GetId(), int(req.GetLimit()))
}

func (s *Service) CronLogs(_ context.Context, req *agentv1.CronLogsRequest) (*agentv1.CronHistoryResponse, error) {
	return s.cronHistory(req.GetId(), int(req.GetLimit()))
}

func (s *Service) cronHistory(id string, limit int) (*agentv1.CronHistoryResponse, error) {
	runs, err := s.cron.History(id, limit)
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.CronHistoryResponse{}
	for _, run := range runs {
		response.Runs = append(response.Runs, cronRunToProto(run))
	}
	return response, nil
}

func (s *Service) CronImport(_ context.Context, req *agentv1.CronImportRequest) (*agentv1.CronListResponse, error) {
	if req.GetFormat() != "json" {
		return nil, status.Error(codes.InvalidArgument, "Agent import accepts normalized JSON only")
	}
	var jobs []agentCron.Job
	if err := json.Unmarshal(req.GetContent(), &jobs); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid cron import")
	}
	if req.GetReplace() {
		current, _ := s.cron.List()
		for _, job := range current {
			_ = s.cron.Delete(job.ID)
		}
	}
	for _, job := range jobs {
		if _, err := s.cron.Create(job); err != nil {
			return nil, cronError(err)
		}
	}
	return s.CronList(context.Background(), &agentv1.CronListRequest{})
}

func (s *Service) CronExport(context.Context, *agentv1.CronExportRequest) (*agentv1.CronExportResponse, error) {
	jobs, err := s.cron.List()
	if err != nil {
		return nil, internalError(err)
	}
	content, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.CronExportResponse{Format: "json", Content: content}, nil
}

func (s *Service) CronValidate(_ context.Context, req *agentv1.CronMutationRequest) (*agentv1.CronValidationResponse, error) {
	result, err := agentCron.ValidateJob(cronJobFromProto(req.GetJob()))
	if err != nil {
		return &agentv1.CronValidationResponse{Valid: false, Error: err.Error()}, nil
	}
	return &agentv1.CronValidationResponse{Valid: result.Valid, Warnings: result.Warnings}, nil
}

func cronJobFromProto(job *agentv1.CronJob) agentCron.Job {
	if job == nil {
		return agentCron.Job{}
	}
	return agentCron.Job{
		ID: job.GetId(), Name: job.GetName(), Description: job.GetDescription(),
		Command: job.GetCommand(), WorkingDir: job.GetWorkingDir(), User: job.GetUser(),
		Environment: job.GetEnvironment(), Expression: job.GetExpression(),
		Timezone: job.GetTimezone(), Enabled: job.GetEnabled(),
		Source: job.GetSource(), ReadOnly: job.GetReadOnly(),
	}
}

func cronJobToProto(job agentCron.Job) *agentv1.CronJob {
	return &agentv1.CronJob{
		Id: job.ID, Name: job.Name, Description: job.Description, Command: job.Command,
		WorkingDir: job.WorkingDir, User: job.User, Environment: job.Environment,
		Expression: job.Expression, Timezone: job.Timezone, Enabled: job.Enabled,
		CreatedAt: job.CreatedAt.UnixMilli(), UpdatedAt: job.UpdatedAt.UnixMilli(),
		Source: job.Source, ReadOnly: job.ReadOnly,
	}
}

func cronRunToProto(run agentCron.Run) *agentv1.CronRunRecord {
	return &agentv1.CronRunRecord{
		Id: run.ID, JobId: run.JobID, StartedAt: run.StartedAt.UnixMilli(),
		FinishedAt: run.FinishedAt.UnixMilli(), DurationMs: run.Duration.Milliseconds(),
		ExitCode: int32(run.ExitCode), Stdout: run.Stdout, Stderr: run.Stderr,
		Triggered: run.Triggered, Actor: run.Actor,
	}
}

func cronError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return status.Error(codes.NotFound, "cron job not found")
	}
	return status.Error(codes.InvalidArgument, err.Error())
}
