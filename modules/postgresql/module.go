// Package postgresql implements the PostgreSQL module for OpenDeploy.
package postgresql

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "postgresql"

// Module is the PostgreSQL OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new PostgreSQL Module.
func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "PostgreSQL" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Powerful, open source object-relational database system" }

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("postgresql module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }
func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "PostgreSQL", Icon: "database", Path: "/modules/postgresql", Order: 35},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key: "port", Label: "Default Port",
			Type: contract.SettingTypeString, DefaultValue: "5432", Required: true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "postgresql: installing")
	ch, err := m.deps.Agent.PackageInstall(ctx, "postgresql")
	if err != nil {
		return fmt.Errorf("postgresql: install: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "postgresql")
	if err != nil {
		return fmt.Errorf("postgresql: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "postgresql"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceStart(ctx, "postgresql")
}

func (m *Module) Disable(ctx context.Context) error {
	_ = m.deps.Agent.ServiceStop(ctx, "postgresql")
	_ = m.deps.Agent.ServiceDisable(ctx, "postgresql")
	return nil
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "postgresql")
}

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "postgresql")
	if !installed {
		return &contract.ModuleStatus{State: contract.StateAvailable}, nil
	}
	svcStatus, _ := m.deps.Agent.ServiceStatus(ctx, "postgresql")
	running := svcStatus != nil && svcStatus.Active
	state := contract.StateDisabled
	if running {
		state = contract.StateEnabled
	}
	return &contract.ModuleStatus{
		State: state, InstalledVersion: version, ServiceRunning: running,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	st, err := m.deps.Agent.ServiceStatus(ctx, "postgresql")
	if err != nil || !st.Active {
		return &contract.HealthReport{
			Status:  contract.HealthWarning,
			Message: "PostgreSQL is not running",
		}, nil
	}
	return &contract.HealthReport{
		Status:  contract.HealthOK,
		Message: "PostgreSQL is running",
	}, nil
}

var _ contract.Module = (*Module)(nil)
