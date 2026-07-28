package nginx

import (
	"context"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "nginx"

// Module is the stable public facade for the Nginx plugin. Operational
// responsibilities are delegated to focused services.
type Module struct {
	deps        contract.ModuleDeps
	log         *slog.Logger
	logsService *LogsService
	gridService *DataGridService
}

func New() *Module {
	module := &Module{}
	module.gridService = NewDataGridService(module, nil)
	return module
}

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Nginx Web Server" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "High-performance HTTP server and reverse proxy" }
func (m *Module) Category() string    { return "Web" }
func (m *Module) Icon() string        { return "server" }

func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}

func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true,
		SupportsSettings: true,
		SupportsLogs:     true,
		SupportsRestart:  true,
		SupportsUpdate:   true,
	}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.log = deps.Logger.With("module", moduleID)
	m.logsService = NewLogsService(deps.Agent)
	m.gridService = NewDataGridService(m, deps.Agent)
	installed, _, err := deps.Agent.PackageInstalled(context.Background(), "nginx")
	if err == nil && !installed {
		m.log.Warn("nginx package is not installed")
	}
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }
func (m *Module) RegisterRoutes(contract.Router) {}
func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{{
		ID: "nginx", Label: "Nginx", Icon: "server", Path: "/modules/nginx", Order: 10,
	}}
}
func (m *Module) RegisterSettings() []contract.SettingSpec { return nil }

func (m *Module) Logs() []contract.LogDef {
	return m.logsService.Logs(context.Background())
}
func (m *Module) ReadLog(ctx context.Context, logID string, lines int) ([]string, error) {
	return m.logsService.Read(ctx, logID, lines)
}
func (m *Module) ClearLog(ctx context.Context, logID string) error {
	return m.logsService.Clear(ctx, logID)
}
func (m *Module) Pages() []contract.ModulePage {
	return m.gridService.Pages(m.Capabilities())
}
func (m *Module) DataGridSchema(pageID string) (contract.DataGridSchema, error) {
	return m.gridService.Schema(pageID)
}
func (m *Module) DataGridData(ctx context.Context, pageID string) ([]map[string]any, error) {
	return m.gridService.Data(ctx, pageID)
}
func (m *Module) DataGridAction(ctx context.Context, pageID, actionID string, payload map[string]any) error {
	return m.gridService.Action(ctx, pageID, actionID, payload)
}

var (
	_ contract.WebServerPlugin  = (*Module)(nil)
	_ contract.DataGridProvider = (*Module)(nil)
)
