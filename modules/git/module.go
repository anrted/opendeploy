// Package git implements the Git module for OpenDeploy.
package git

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "git"

// Module is the Git OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new Git Module.
func New() *Module { return &Module{} }

func (m *Module) ID() string      { return moduleID }
func (m *Module) Name() string    { return "Git" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Description() string {
	return "Distributed version control system for automated deployments"
}

func (m *Module) Category() string { return "Development" }
func (m *Module) Icon() string     { return "git-branch" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  false,
		SupportsSettings: true,
		SupportsLogs:     false,
		SupportsRestart:  false,
		SupportsUpdate:   true,
	}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("git module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }
func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "Git", Icon: "git-branch", Path: "/modules/git", Order: 40},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key: "deploy_path", Label: "Default Deploy Path",
			Type: contract.SettingTypeString, DefaultValue: "/var/www",
			Description: "Root directory where repositories are deployed",
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageInstall(ctx, "git")
	if err != nil {
		return fmt.Errorf("git: install: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "git")
	if err != nil {
		return fmt.Errorf("git: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

// Git is not a daemon — Enable/Disable/Restart are no-ops.
func (m *Module) Enable(_ context.Context) error  { return nil }
func (m *Module) Disable(_ context.Context) error { return nil }
func (m *Module) Restart(_ context.Context) error { return nil }

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "git")
	status := contract.PackageNotInstalled
	if installed {
		status = contract.PackageInstalled
	}
	return &contract.RuntimeStatus{
		PackageStatus:   status,
		SoftwareVersion: version,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "git")
	if !installed {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "git is not installed"}, nil
	}
	return &contract.HealthReport{
		Status:  contract.HealthOK,
		Message: fmt.Sprintf("git %s is installed", version),
	}, nil
}

var _ contract.Module = (*Module)(nil)

func (m *Module) Actions() []contract.ActionDef { return nil }
func (m *Module) ExecuteAction(ctx context.Context, actionID string) error {
	return fmt.Errorf("unknown action: %s", actionID)
}
func (m *Module) Logs() []contract.LogDef {
	if m.Capabilities().SupportsService {
		return []contract.LogDef{{ID: "service", Name: "Systemd Log", Type: "systemd"}}
	}
	return nil
}
func (m *Module) SettingsSchema() []contract.SettingField { return nil }

func (m *Module) Pages() []contract.ModulePage {
	pages := []contract.ModulePage{
		{ID: "overview", Title: "Overview", Type: contract.PageTypeOverview},
	}
	if m.Capabilities().SupportsSettings {
		pages = append(pages, contract.ModulePage{ID: "settings", Title: "Settings", Type: contract.PageTypeSettings})
	}
	if m.Capabilities().SupportsLogs {
		pages = append(pages, contract.ModulePage{ID: "logs", Title: "Logs", Type: contract.PageTypeLogs})
	}
	return pages
}
