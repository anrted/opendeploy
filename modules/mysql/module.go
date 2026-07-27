package mysql

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "mysql"

type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "MySQL Database" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Relational database management system" }

func (m *Module) Category() string { return "Databases" }
func (m *Module) Icon() string     { return "database" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true,
		SupportsSettings: false,
		SupportsLogs:     true,
		SupportsRestart:  true,
		SupportsUpdate:   true,
	}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	return nil
}

func (m *Module) Shutdown(_ context.Context) error       { return nil }
func (m *Module) RegisterRoutes(_ contract.Router)       {}
func (m *Module) RegisterMenuItems() []contract.MenuItem { return nil }

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key:         "root_password",
			Label:       "Root Password",
			Description: "Database root password",
			Type:        contract.SettingTypeString,
			Secret:      true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageInstall(ctx, "mysql-server")
	if err != nil {
		return err
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "mysql-server")
	if err != nil {
		return err
	}
	for range ch {
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "mysql"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceStart(ctx, "mysql")
}

func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "mysql"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceDisable(ctx, "mysql")
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "mysql")
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "mysql-server")
	pkgStatus := contract.PackageNotInstalled
	if installed {
		pkgStatus = contract.PackageInstalled
	}

	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "mysql")
	var srvStatus contract.ServiceStatusState = contract.ServiceStopped
	if err == nil && svcStatus != nil {
		if svcStatus.Active {
			srvStatus = contract.ServiceRunning
		}
	} else if err != nil {
		srvStatus = contract.ServiceFailed
	}

	return &contract.RuntimeStatus{
		PackageStatus:   pkgStatus,
		ServiceStatus:   srvStatus,
		SoftwareVersion: version,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	installed, version, err := m.deps.Agent.PackageInstalled(ctx, "mysql-server")
	if err != nil {
		return &contract.HealthReport{Status: contract.HealthError, Message: "cannot query MySQL package state"}, nil
	}
	if !installed {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "MySQL is not installed"}, nil
	}
	serviceStatus, err := m.deps.Agent.ServiceStatus(ctx, "mysql")
	if err != nil {
		return &contract.HealthReport{Status: contract.HealthError, Message: "cannot query MySQL service state"}, nil
	}
	status := contract.HealthOK
	message := fmt.Sprintf("MySQL %s is installed and running", version)
	if !serviceStatus.Active {
		status = contract.HealthError
		message = fmt.Sprintf("MySQL %s is installed but not running", version)
	}
	return &contract.HealthReport{
		Status:  status,
		Message: message,
		Checks: []contract.HealthCheck{
			{Name: "package_installed", Status: contract.HealthOK, Message: version},
			{Name: "service_running", Status: status, Message: serviceStatus.SubState},
		},
	}, nil
}

// ─── DatabasePlugin ────────────────────────────────────────────────────────

var _ contract.DatabasePlugin = (*Module)(nil)

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
