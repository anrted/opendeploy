package capability

import (
	"context"

	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/pkg/contract"
)

func (c *Client) localCron() contract.CronAgentClient {
	return c.local.(contract.CronAgentClient)
}

func (c *Client) CronList(ctx context.Context) ([]contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronList(ctx)
	}
	var result []contract.CronJob
	err := c.call(ctx, "cron.list", struct{}{}, &result)
	return result, err
}
func (c *Client) CronGet(ctx context.Context, id string) (*contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronGet(ctx, id)
	}
	var result contract.CronJob
	err := c.call(ctx, "cron.get", map[string]string{"id": id}, &result)
	return &result, err
}
func (c *Client) CronCreate(ctx context.Context, job contract.CronJob) (*contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronCreate(ctx, job)
	}
	var result contract.CronJob
	err := c.call(ctx, "cron.create", job, &result)
	return &result, err
}
func (c *Client) CronUpdate(ctx context.Context, job contract.CronJob) (*contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronUpdate(ctx, job)
	}
	var result contract.CronJob
	err := c.call(ctx, "cron.update", job, &result)
	return &result, err
}
func (c *Client) CronDelete(ctx context.Context, id string) error {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronDelete(ctx, id)
	}
	return c.call(ctx, "cron.delete", map[string]string{"id": id}, nil)
}
func (c *Client) CronEnable(ctx context.Context, id string) (*contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronEnable(ctx, id)
	}
	var result contract.CronJob
	err := c.call(ctx, "cron.enable", map[string]string{"id": id}, &result)
	return &result, err
}
func (c *Client) CronDisable(ctx context.Context, id string) (*contract.CronJob, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronDisable(ctx, id)
	}
	var result contract.CronJob
	err := c.call(ctx, "cron.disable", map[string]string{"id": id}, &result)
	return &result, err
}
func (c *Client) CronRun(ctx context.Context, id, trigger, actor string) (*contract.CronRun, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronRun(ctx, id, trigger, actor)
	}
	var result contract.CronRun
	err := c.call(ctx, "cron.run", map[string]string{"id": id, "trigger": trigger, "actor": actor}, &result)
	return &result, err
}
func (c *Client) CronHistory(ctx context.Context, id string, limit int) ([]contract.CronRun, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronHistory(ctx, id, limit)
	}
	var result []contract.CronRun
	err := c.call(ctx, "cron.history", map[string]any{"id": id, "limit": limit}, &result)
	return result, err
}
func (c *Client) CronValidate(ctx context.Context, job contract.CronJob) (*contract.CronValidation, error) {
	if servercontext.IsLocal(ctx) {
		return c.localCron().CronValidate(ctx, job)
	}
	var result contract.CronValidation
	err := c.call(ctx, "cron.validate", job, &result)
	return &result, err
}

var _ contract.CronAgentClient = (*Client)(nil)
