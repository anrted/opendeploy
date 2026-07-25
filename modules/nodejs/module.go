// Package nodejs implements the Node.js module for OpenDeploy.
package nodejs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "nodejs"

// Module is the Node.js OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new Node.js Module.
func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Node.js" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "JavaScript runtime environment (LTS)" }

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("nodejs module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }
func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "Node.js", Icon: "terminal", Path: "/modules/nodejs", Order: 30},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key: "node_version", Label: "Node.js Version",
			Type:         contract.SettingTypeSelect,
			Options:      []string{"18", "20", "22"},
			DefaultValue: "20", Required: true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "nodejs: installing nodejs LTS")
	ch, err := m.deps.Agent.PackageInstall(ctx, "nodejs")
	if err != nil {
		return fmt.Errorf("nodejs: install: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "nodejs")
	if err != nil {
		return fmt.Errorf("nodejs: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

// Node.js itself is not a system service; Enable/Disable manage pm2 if available.
func (m *Module) Enable(_ context.Context) error  { return nil }
func (m *Module) Disable(_ context.Context) error { return nil }
func (m *Module) Restart(_ context.Context) error { return nil }

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "nodejs")
	if !installed {
		return &contract.ModuleStatus{State: contract.StateAvailable}, nil
	}
	return &contract.ModuleStatus{
		State: contract.StateInstalled, InstalledVersion: version,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "nodejs")
	if !installed {
		return &contract.HealthReport{
			Status:  contract.HealthWarning,
			Message: "Node.js is not installed",
		}, nil
	}
	return &contract.HealthReport{
		Status:  contract.HealthOK,
		Message: fmt.Sprintf("Node.js %s is installed", version),
	}, nil
}

var _ contract.Module = (*Module)(nil)
