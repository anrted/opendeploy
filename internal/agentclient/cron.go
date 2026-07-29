package agentclient

import (
	"context"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

func (c *Client) CronList(ctx context.Context) ([]contract.CronJob, error) {
	response, err := c.stub.CronList(ctx, &agentv1.CronListRequest{})
	if err != nil {
		return nil, err
	}
	jobs := make([]contract.CronJob, 0, len(response.GetJobs()))
	for _, job := range response.GetJobs() {
		jobs = append(jobs, cronJobFromProto(job))
	}
	return jobs, nil
}

func (c *Client) CronGet(ctx context.Context, id string) (*contract.CronJob, error) {
	job, err := c.stub.CronGet(ctx, &agentv1.CronGetRequest{Id: id})
	if err != nil {
		return nil, err
	}
	result := cronJobFromProto(job)
	return &result, nil
}

func (c *Client) CronCreate(ctx context.Context, job contract.CronJob) (*contract.CronJob, error) {
	result, err := c.stub.CronCreate(ctx, &agentv1.CronMutationRequest{Job: cronJobToProto(job)})
	if err != nil {
		return nil, err
	}
	converted := cronJobFromProto(result)
	return &converted, nil
}

func (c *Client) CronUpdate(ctx context.Context, job contract.CronJob) (*contract.CronJob, error) {
	result, err := c.stub.CronUpdate(ctx, &agentv1.CronMutationRequest{Job: cronJobToProto(job)})
	if err != nil {
		return nil, err
	}
	converted := cronJobFromProto(result)
	return &converted, nil
}

func (c *Client) CronDelete(ctx context.Context, id string) error {
	_, err := c.stub.CronDelete(ctx, &agentv1.CronIDRequest{Id: id})
	return err
}

func (c *Client) CronEnable(ctx context.Context, id string) (*contract.CronJob, error) {
	result, err := c.stub.CronEnable(ctx, &agentv1.CronIDRequest{Id: id})
	if err != nil {
		return nil, err
	}
	converted := cronJobFromProto(result)
	return &converted, nil
}

func (c *Client) CronDisable(ctx context.Context, id string) (*contract.CronJob, error) {
	result, err := c.stub.CronDisable(ctx, &agentv1.CronIDRequest{Id: id})
	if err != nil {
		return nil, err
	}
	converted := cronJobFromProto(result)
	return &converted, nil
}

func (c *Client) CronRun(ctx context.Context, id, trigger, actor string) (*contract.CronRun, error) {
	result, err := c.stub.CronRun(ctx, &agentv1.CronRunRequest{Id: id, Trigger: trigger, Actor: actor})
	if err != nil {
		return nil, err
	}
	converted := cronRunFromProto(result)
	return &converted, nil
}

func (c *Client) CronHistory(ctx context.Context, id string, limit int) ([]contract.CronRun, error) {
	if limit < 0 {
		limit = 0
	}
	// Keep literal bounds: CodeQL can prove this architecture-dependent int
	// conversion safe only when the int32 maximum is compared as a constant.
	if limit > 2147483647 {
		limit = 2147483647
	}
	response, err := c.stub.CronHistory(ctx, &agentv1.CronHistoryRequest{Id: id, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	runs := make([]contract.CronRun, 0, len(response.GetRuns()))
	for _, run := range response.GetRuns() {
		runs = append(runs, cronRunFromProto(run))
	}
	return runs, nil
}

func (c *Client) CronValidate(ctx context.Context, job contract.CronJob) (*contract.CronValidation, error) {
	response, err := c.stub.CronValidate(ctx, &agentv1.CronMutationRequest{Job: cronJobToProto(job)})
	if err != nil {
		return nil, err
	}
	return &contract.CronValidation{Valid: response.GetValid(), Warnings: response.GetWarnings(), Error: response.GetError()}, nil
}

func cronJobToProto(job contract.CronJob) *agentv1.CronJob {
	return &agentv1.CronJob{
		Id: job.ID, Name: job.Name, Description: job.Description, Command: job.Command,
		WorkingDir: job.WorkingDir, User: job.User, Environment: job.Environment,
		Expression: job.Expression, Timezone: job.Timezone, Enabled: job.Enabled,
		Source: job.Source, ReadOnly: job.ReadOnly,
	}
}

func cronJobFromProto(job *agentv1.CronJob) contract.CronJob {
	return contract.CronJob{
		ID: job.GetId(), Name: job.GetName(), Description: job.GetDescription(),
		Command: job.GetCommand(), WorkingDir: job.GetWorkingDir(), User: job.GetUser(),
		Environment: job.GetEnvironment(), Expression: job.GetExpression(),
		Timezone: job.GetTimezone(), Enabled: job.GetEnabled(),
		Source: job.GetSource(), ReadOnly: job.GetReadOnly(),
		CreatedAt: time.UnixMilli(job.GetCreatedAt()).UTC(), UpdatedAt: time.UnixMilli(job.GetUpdatedAt()).UTC(),
	}
}

func cronRunFromProto(run *agentv1.CronRunRecord) contract.CronRun {
	return contract.CronRun{
		ID: run.GetId(), JobID: run.GetJobId(), StartedAt: time.UnixMilli(run.GetStartedAt()).UTC(),
		FinishedAt: time.UnixMilli(run.GetFinishedAt()).UTC(), Duration: time.Duration(run.GetDurationMs()) * time.Millisecond,
		ExitCode: int(run.GetExitCode()), Stdout: run.GetStdout(), Stderr: run.GetStderr(),
		Triggered: run.GetTriggered(), Actor: run.GetActor(),
	}
}
