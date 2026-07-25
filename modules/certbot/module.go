// Package certbot implements the Certbot module for OpenDeploy.
package certbot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "certbot"

// Module is the Certbot OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new Certbot Module.
func New() *Module { return &Module{} }

func (m *Module) ID() string      { return moduleID }
func (m *Module) Name() string    { return "Certbot" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Description() string {
	return "Let's Encrypt client and ACME tool for SSL certificates"
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("certbot module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }
func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "Certbot", Icon: "shield", Path: "/modules/certbot", Order: 50},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key: "email", Label: "Let's Encrypt Email",
			Type: contract.SettingTypeString, DefaultValue: "",
			Description: "Email address for important account notifications",
			Required: true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageInstall(ctx, "certbot")
	if err != nil {
		return fmt.Errorf("certbot: install: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "certbot")
	if err != nil {
		return fmt.Errorf("certbot: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

// Enable starts and enables the certbot renewal timer.
func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "certbot.timer"); err != nil {
		return fmt.Errorf("certbot: enable timer: %w", err)
	}
	return m.deps.Agent.ServiceStart(ctx, "certbot.timer")
}

// Disable stops and disables the certbot renewal timer.
func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "certbot.timer"); err != nil {
		return fmt.Errorf("certbot: stop timer: %w", err)
	}
	return m.deps.Agent.ServiceDisable(ctx, "certbot.timer")
}

// Restart reloads the certbot timer.
func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "certbot.timer")
}

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	// Check if installed
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "certbot")
	if !installed {
		return &contract.ModuleStatus{State: contract.StateAvailable}, nil
	}

	state := contract.StateInstalled
	// Check timer status to see if it's "enabled"
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "certbot.timer")
	serviceRunning := false
	if err == nil && svcStatus.Active {
		state = contract.StateEnabled
		serviceRunning = true
	}

	return &contract.ModuleStatus{
		State:            state,
		InstalledVersion: version,
		ServiceRunning:   serviceRunning,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "certbot")
	if !installed {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "certbot is not installed"}, nil
	}
	
	// Check if timer is active
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "certbot.timer")
	if err != nil || !svcStatus.Active {
		return &contract.HealthReport{
			Status:  contract.HealthWarning,
			Message: fmt.Sprintf("certbot %s is installed, but timer is not active", version),
		}, nil
	}

	return &contract.HealthReport{
		Status:  contract.HealthOK,
		Message: fmt.Sprintf("certbot %s is installed and timer is active", version),
	}, nil
}

var _ contract.Module = (*Module)(nil)
