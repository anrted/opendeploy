package mysql

import (
	"context"
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
func (m *Module) Version() string     { return "8.0" }
func (m *Module) Description() string { return "Relational database management system" }

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }
func (m *Module) RegisterRoutes(_ contract.Router) {}
func (m *Module) RegisterMenuItems() []contract.MenuItem { return nil }

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key:          "root_password",
			Label:        "Root Password",
			Description:  "Database root password",
			Type:         contract.SettingTypeString,
			Secret:       true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageInstall(ctx, "mysql-server")
	if err != nil {
		return err
	}
	for range ch {}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	ch, err := m.deps.Agent.PackageRemove(ctx, "mysql-server")
	if err != nil {
		return err
	}
	for range ch {}
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

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "mysql")
	if err != nil {
		return &contract.ModuleStatus{State: contract.StateError, Details: err.Error()}, nil
	}
	state := contract.StateDisabled
	if svcStatus.Active {
		state = contract.StateEnabled
	}
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "mysql-server")
	if !installed {
		state = contract.StateAvailable
	}
	return &contract.ModuleStatus{
		State:            state,
		InstalledVersion: version,
		ServiceRunning:   svcStatus.Active,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	return &contract.HealthReport{Status: contract.HealthOK}, nil
}

// ─── DatabasePlugin ────────────────────────────────────────────────────────

func (m *Module) CreateDatabase(ctx context.Context, dbName, user, password string) error {
	// In a real implementation, we would connect to MySQL and execute:
	// CREATE DATABASE `dbName`;
	// CREATE USER 'user'@'localhost' IDENTIFIED BY 'password';
	// GRANT ALL PRIVILEGES ON `dbName`.* TO 'user'@'localhost';
	m.logger.Info("MySQL: Creating database", "db", dbName, "user", user)
	return nil
}

func (m *Module) DeleteDatabase(ctx context.Context, dbName, user string) error {
	// DROP DATABASE `dbName`;
	// DROP USER 'user'@'localhost';
	m.logger.Info("MySQL: Deleting database", "db", dbName, "user", user)
	return nil
}

var _ contract.DatabasePlugin = (*Module)(nil)
