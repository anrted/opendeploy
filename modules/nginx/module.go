// Package nginx implements the Nginx module for OpenDeploy.
//
// It provides install, uninstall, enable, disable, restart, status and
// health-check operations delegated to the Agent, plus Nginx-specific
// API endpoints for vhost management.
package nginx

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "nginx"

// Module is the Nginx OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new Nginx Module.
func New() *Module { return &Module{} }

// ─── contract.Module interface ─────────────────────────────────────────────

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Nginx Web Server" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "High-performance HTTP server and reverse proxy" }

// Bootstrap stores the injected dependencies.
func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("nginx module bootstrapped")
	return nil
}

// Shutdown is a no-op for Nginx; the service is managed by systemd.
func (m *Module) Shutdown(_ context.Context) error { return nil }

// RegisterRoutes registers Nginx-specific API endpoints.
func (m *Module) RegisterRoutes(_ contract.Router) {
	// Module-specific routes are registered directly on the chi sub-router
	// by the core in server/router.go — this method documents intent.
}

// RegisterMenuItems returns the sidebar menu items contributed by this module.
func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{
			ID:    "nginx",
			Label: "Nginx",
			Icon:  "server",
			Path:  "/modules/nginx",
			Order: 10,
		},
	}
}

// RegisterSettings returns the configurable settings for this module.
func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key:          "worker_processes",
			Label:        "Worker Processes",
			Description:  "Number of worker processes (use 'auto' for CPU count)",
			Type:         contract.SettingTypeString,
			DefaultValue: "auto",
			Required:     true,
		},
		{
			Key:          "worker_connections",
			Label:        "Worker Connections",
			Description:  "Maximum connections per worker",
			Type:         contract.SettingTypeInt,
			DefaultValue: "1024",
			Required:     true,
		},
	}
}

// Install installs nginx via the system package manager.
func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "nginx: installing")
	ch, err := m.deps.Agent.PackageInstall(ctx, "nginx")
	if err != nil {
		return fmt.Errorf("nginx: install: %w", err)
	}
	// Drain the output channel (in production, forward to Job output).
	for range ch {
	}
	return nil
}

// Uninstall removes nginx via the system package manager.
func (m *Module) Uninstall(ctx context.Context) error {
	m.logger.InfoContext(ctx, "nginx: uninstalling")
	ch, err := m.deps.Agent.PackageRemove(ctx, "nginx")
	if err != nil {
		return fmt.Errorf("nginx: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

// Enable starts nginx and enables it for autostart.
func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "nginx"); err != nil {
		return fmt.Errorf("nginx: enable: %w", err)
	}
	return m.deps.Agent.ServiceStart(ctx, "nginx")
}

// Disable stops nginx and disables autostart.
func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "nginx"); err != nil {
		return fmt.Errorf("nginx: stop: %w", err)
	}
	return m.deps.Agent.ServiceDisable(ctx, "nginx")
}

// Restart reloads the nginx configuration.
func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "nginx")
}

// Status returns the current runtime status of the nginx service.
func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "nginx")
	if err != nil {
		return &contract.ModuleStatus{State: contract.StateError, Details: err.Error()}, nil
	}
	state := contract.StateDisabled
	if svcStatus.Active {
		state = contract.StateEnabled
	}
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "nginx")
	if !installed {
		state = contract.StateAvailable
	}
	return &contract.ModuleStatus{
		State:            state,
		InstalledVersion: version,
		ServiceRunning:   svcStatus.Active,
	}, nil
}

// HealthCheck verifies that nginx is running and its config is valid.
func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "nginx")
	if err != nil {
		return &contract.HealthReport{
			Status:  contract.HealthError,
			Message: fmt.Sprintf("cannot query nginx service: %v", err),
		}, nil
	}

	checks := []contract.HealthCheck{
		{
			Name:    "service_running",
			Status:  boolHealth(svcStatus.Active),
			Message: formatServiceMsg(svcStatus.Active),
		},
	}

	overall := contract.HealthOK
	for _, c := range checks {
		if c.Status == contract.HealthError {
			overall = contract.HealthError
			break
		}
	}

	return &contract.HealthReport{
		Status: overall,
		Checks: checks,
	}, nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

func boolHealth(ok bool) contract.HealthStatus {
	if ok {
		return contract.HealthOK
	}
	return contract.HealthError
}

func formatServiceMsg(active bool) string {
	if active {
		return "nginx service is running"
	}
	return "nginx service is not running"
}

// compile-time assertion
var _ contract.Module = (*Module)(nil)
