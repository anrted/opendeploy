package cron

import (
	"context"
	"fmt"
	"strings"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

type Module struct {
	agent contract.CronAgentClient
	base  contract.AgentClient
	tasks contract.TaskRunner
}

func New() *Module { return &Module{} }

func (m *Module) ID() string          { return "cron" }
func (m *Module) Name() string        { return "Cron Scheduler" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Schedule, run and audit recurring server tasks" }
func (m *Module) Category() string    { return "System" }
func (m *Module) Icon() string        { return "clock" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{SupportsService: true, SupportsSettings: true, SupportsLogs: true, SupportsRestart: true}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	agent, ok := deps.Agent.(contract.CronAgentClient)
	if !ok {
		return fmt.Errorf("connected Agent does not support Cron RPC")
	}
	m.agent, m.base = agent, deps.Agent
	m.tasks = deps.Tasks
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }
func (m *Module) RegisterRoutes(router contract.Router) {
	m.registerRoutes(router)
}
func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{{ID: "cron", Label: "Cron", Icon: "clock", Path: "/cron", Order: 55}}
}
func (m *Module) RegisterSettings() []contract.SettingSpec { return nil }

func (m *Module) Install(ctx context.Context) error {
	output, err := m.base.PackageInstall(ctx, "cron")
	if err != nil {
		return err
	}
	for range output {
	}
	return nil
}
func (m *Module) Uninstall(ctx context.Context) error {
	output, err := m.base.PackageRemove(ctx, "cron")
	if err != nil {
		return err
	}
	for range output {
	}
	return nil
}
func (m *Module) Enable(ctx context.Context) error  { return m.base.ServiceEnable(ctx, "cron") }
func (m *Module) Disable(ctx context.Context) error { return m.base.ServiceDisable(ctx, "cron") }
func (m *Module) Restart(ctx context.Context) error { return m.base.ServiceRestart(ctx, "cron") }

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	status, err := m.base.ServiceStatus(ctx, "cron")
	if err != nil {
		status, err = m.base.ServiceStatus(ctx, "crond")
	}
	if err != nil {
		return &contract.RuntimeStatus{PackageStatus: contract.PackageNotInstalled, Health: contract.HealthWarning, Details: "cron daemon not found"}, nil
	}
	serviceState := contract.ServiceStopped
	moduleState := contract.HealthWarning
	if status.Active {
		serviceState, moduleState = contract.ServiceRunning, contract.HealthOK
	}
	return &contract.RuntimeStatus{
		PackageStatus: contract.PackageInstalled, ServiceStatus: serviceState, SoftwareVersion: "system cron",
		Health: moduleState, Properties: []contract.Property{{Name: "daemon", Value: status.Name}, {Name: "sub_state", Value: status.SubState}},
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	status, err := m.Status(ctx)
	if err != nil {
		return nil, err
	}
	check := contract.HealthCheck{Name: "daemon", Status: status.Health, Message: status.Details}
	if status.ServiceStatus == contract.ServiceRunning {
		check.Message = "cron daemon is running"
	}
	return &contract.HealthReport{Status: status.Health, Message: check.Message, Checks: []contract.HealthCheck{check}}, nil
}

func (m *Module) Pages() []contract.ModulePage {
	return []contract.ModulePage{
		{ID: "overview", Title: "Overview", Type: contract.PageTypeOverview},
		{ID: "jobs", Title: "Cron Jobs", Type: contract.PageTypeDataGrid},
		{ID: "history", Title: "History", Type: contract.PageTypeDataGrid},
		{ID: "settings", Title: "Settings", Type: contract.PageTypeSettings},
	}
}
func (m *Module) Actions() []contract.ActionDef { return nil }
func (m *Module) ExecuteAction(context.Context, string) error {
	return fmt.Errorf("unknown action")
}
func (m *Module) Logs() []contract.LogDef { return nil }
func (m *Module) SettingsSchema() []contract.SettingField {
	return []contract.SettingField{
		{ID: "crontab_path", Type: "string", Label: "Managed crontab", Value: "/etc/cron.d/opendeploy", Category: "General"},
		{ID: "daemon", Type: "select", Label: "Cron daemon", Value: "auto", Options: []string{"auto", "cron", "crond"}, Category: "General"},
		{ID: "history_retention", Type: "number", Label: "History retention", Value: 500, Category: "History"},
		{ID: "max_log_size", Type: "number", Label: "Maximum output bytes", Value: 1048576, Category: "History"},
	}
}
func (m *Module) SaveSettings(context.Context, map[string]any) error {
	return nil
}

func (m *Module) DataGridSchema(pageID string) (contract.DataGridSchema, error) {
	switch pageID {
	case "jobs":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "name", Title: "Name", Type: "text", Sortable: true},
				{Key: "command", Title: "Command", Type: "text"},
				{Key: "user", Title: "User", Type: "text", Sortable: true},
				{Key: "expression", Title: "Schedule", Type: "text", Sortable: true},
				{Key: "status", Title: "Status", Type: "badge", Sortable: true},
				{Key: "last_run", Title: "Last Run", Type: "date", Sortable: true},
				{Key: "exit_code", Title: "Exit Code", Type: "number"},
			},
			Actions: []contract.ActionDef{{ID: "create", Title: "Create Cron Job", Icon: "plus", Color: "primary"}},
			RowActions: []contract.ActionDef{
				{ID: "run", Title: "Run Now", Icon: "play", Color: "primary", RequiresConfirmation: true},
				{ID: "enable", Title: "Enable", Icon: "play", Color: "success"},
				{ID: "disable", Title: "Disable", Icon: "pause", Color: "warning"},
				{ID: "duplicate", Title: "Duplicate", Icon: "copy", Color: "primary"},
				{ID: "delete", Title: "Delete", Icon: "trash-2", Color: "danger", Dangerous: true, RequiresConfirmation: true},
			},
		}, nil
	case "history":
		return contract.DataGridSchema{Columns: []contract.DataGridColumn{
			{Key: "started_at", Title: "Date", Type: "date", Sortable: true},
			{Key: "job_id", Title: "Job", Type: "text", Sortable: true},
			{Key: "status", Title: "Status", Type: "badge", Sortable: true},
			{Key: "duration_ms", Title: "Duration", Type: "number"},
			{Key: "exit_code", Title: "Exit Code", Type: "number"},
			{Key: "triggered", Title: "Trigger", Type: "badge"},
			{Key: "actor", Title: "Actor", Type: "text"},
		}}, nil
	default:
		return contract.DataGridSchema{}, fmt.Errorf("unknown cron page %q", pageID)
	}
}

func (m *Module) DataGridData(ctx context.Context, pageID string) ([]map[string]any, error) {
	if pageID == "history" {
		runs, err := m.agent.CronHistory(ctx, "", 500)
		if err != nil {
			return nil, apperrors.AgentUnavailable(err)
		}
		rows := make([]map[string]any, 0, len(runs))
		for _, run := range runs {
			status := "success"
			if run.ExitCode != 0 {
				status = "failed"
			}
			rows = append(rows, map[string]any{
				"id": run.ID, "job_id": run.JobID, "started_at": run.StartedAt, "status": status,
				"duration_ms": run.Duration.Milliseconds(), "exit_code": run.ExitCode,
				"triggered": run.Triggered, "actor": run.Actor, "stdout": run.Stdout, "stderr": run.Stderr,
			})
		}
		return rows, nil
	}
	if pageID != "jobs" {
		return nil, fmt.Errorf("unknown cron page %q", pageID)
	}
	jobs, err := m.agent.CronList(ctx)
	if err != nil {
		return nil, err
	}
	runs, _ := m.agent.CronHistory(ctx, "", 500)
	rows := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		var last *contract.CronRun
		for index := range runs {
			if runs[index].JobID == job.ID {
				last = &runs[index]
				break
			}
		}
		row := map[string]any{
			"id": job.ID, "name": job.Name, "description": job.Description, "command": job.Command,
			"working_dir": job.WorkingDir, "user": job.User, "environment": job.Environment,
			"expression": job.Expression, "timezone": job.Timezone, "enabled": job.Enabled,
			"status": map[bool]string{true: "enabled", false: "disabled"}[job.Enabled],
			"source": job.Source, "read_only": job.ReadOnly,
		}
		if last != nil {
			row["last_run"], row["exit_code"] = last.StartedAt, last.ExitCode
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (m *Module) DataGridAction(ctx context.Context, pageID, actionID string, payload map[string]any) error {
	if pageID != "jobs" {
		return fmt.Errorf("actions are not supported on %s", pageID)
	}
	id := strings.TrimSpace(fmt.Sprint(payload["id"]))
	switch actionID {
	case "run":
		_, err := m.agent.CronRun(ctx, id, "manual", "")
		return err
	case "enable":
		_, err := m.agent.CronEnable(ctx, id)
		return err
	case "disable":
		_, err := m.agent.CronDisable(ctx, id)
		return err
	case "delete":
		return m.agent.CronDelete(ctx, id)
	case "duplicate":
		job, err := m.agent.CronGet(ctx, id)
		if err != nil {
			return err
		}
		job.ID, job.Name, job.Enabled = job.ID+"-copy", job.Name+" (copy)", false
		_, err = m.agent.CronCreate(ctx, *job)
		return err
	default:
		return fmt.Errorf("unknown cron action %q", actionID)
	}
}

var (
	_ contract.Module           = (*Module)(nil)
	_ contract.DataGridProvider = (*Module)(nil)
	_ contract.SettingsProvider = (*Module)(nil)
)
