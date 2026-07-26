package nginx

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"text/template"

	"github.com/anrted/opendeploy/pkg/contract"
)

//go:embed templates/site.conf.tmpl
var siteConfigTmpl string
var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("site").Parse(siteConfigTmpl))
}

const moduleID = "nginx"

// Module implements the Nginx module.
type Module struct {
	deps contract.ModuleDeps
	log  *slog.Logger
}

// New constructs the Nginx module.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Nginx Web Server" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "High-performance HTTP server and reverse proxy" }

func (m *Module) Category() string { return "Web" }
func (m *Module) Icon() string     { return "server" }
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
	m.log = deps.Logger.With("module", moduleID)

	// Optionally verify nginx is installed.
	installed, _, err := deps.Agent.PackageInstalled(context.Background(), "nginx")
	if err == nil && !installed {
		m.log.Warn("nginx package is not installed")
	}

	return nil
}

func (m *Module) Shutdown(ctx context.Context) error { return nil }

func (m *Module) RegisterRoutes(r contract.Router) {}

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

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return nil
}

func (m *Module) Install(ctx context.Context) error {
	m.log.InfoContext(ctx, "installing nginx")
	out, err := m.deps.Agent.PackageInstall(ctx, "nginx")
	if err != nil {
		return err
	}
	for line := range out {
		m.log.DebugContext(ctx, "apt-get: "+line)
	}
	return m.deps.Agent.ServiceEnable(ctx, "nginx")
}

func (m *Module) Uninstall(ctx context.Context) error {
	m.log.InfoContext(ctx, "uninstalling nginx")
	_ = m.deps.Agent.ServiceStop(ctx, "nginx")
	out, err := m.deps.Agent.PackageRemove(ctx, "nginx")
	if err != nil {
		return err
	}
	for line := range out {
		m.log.DebugContext(ctx, "apt-get: "+line)
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "nginx"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceStart(ctx, "nginx")
}

func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "nginx"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceDisable(ctx, "nginx")
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "nginx")
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "nginx")

	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "nginx")
	pkgStatus := contract.PackageNotInstalled
	if installed {
		pkgStatus = contract.PackageInstalled
	}

	var srvStatus contract.ServiceStatusState = contract.ServiceStopped
	if err == nil {
		if svcStatus.Active {
			srvStatus = contract.ServiceRunning
		}
	} else {
		srvStatus = contract.ServiceFailed
	}

	return &contract.RuntimeStatus{
		PackageStatus:   pkgStatus,
		ServiceStatus:   srvStatus,
		SoftwareVersion: version,
		Details:         "",
	}, nil
}

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
	if !svcStatus.Active {
		overall = contract.HealthError
	}

	return &contract.HealthReport{
		Status: overall,
		Checks: checks,
	}, nil
}

// ─── WebServerPlugin ────────────────────────────────────────────────────────

func (m *Module) ApplySite(ctx context.Context, action contract.SiteAction, site contract.SiteSpec) error {
	configPath := fmt.Sprintf("/etc/nginx/sites-available/opendeploy-%s.conf", site.PrimaryDomain)
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/opendeploy-%s.conf", site.PrimaryDomain)

	switch action {
	case contract.SiteUpsert, contract.SiteEnable:
		if action == contract.SiteUpsert {
			content, err := renderNginx(site)
			if err != nil {
				return err
			}
			if err := m.deps.Agent.FileWrite(ctx, configPath, content, 0o644); err != nil {
				return fmt.Errorf("nginx: write config: %w", err)
			}
			if err := m.deps.Agent.FileWrite(ctx, enabledPath, content, 0o644); err != nil {
				return fmt.Errorf("nginx: enable site: %w", err)
			}
		}

		// Test configuration
		exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil || exitCode != 0 {
			// Rollback
			_ = m.deps.Agent.FileDelete(ctx, enabledPath)
			if action == contract.SiteUpsert {
				_ = m.deps.Agent.FileDelete(ctx, configPath)
			}
			return fmt.Errorf("nginx config test failed: %s\n%s", stdout, stderr)
		}

		// Reload configuration safely
		return m.deps.Agent.ServiceReload(ctx, "nginx")

	case contract.SiteDisable:
		_ = m.deps.Agent.FileDelete(ctx, enabledPath)
		return m.deps.Agent.ServiceReload(ctx, "nginx")

	case contract.SiteDelete:
		_ = m.deps.Agent.FileDelete(ctx, enabledPath)
		_ = m.deps.Agent.FileDelete(ctx, configPath)
		return m.deps.Agent.ServiceReload(ctx, "nginx")
	}

	return nil
}

func renderNginx(site contract.SiteSpec) ([]byte, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, site); err != nil {
		return nil, fmt.Errorf("failed to render nginx template: %w", err)
	}
	return b.Bytes(), nil
}

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
var _ contract.WebServerPlugin = (*Module)(nil)

func (m *Module) Actions() []contract.ActionDef {
	return []contract.ActionDef{
		{ID: "reload", Title: "Reload Configuration", Icon: "refresh", Color: "primary", RequiresConfirmation: false},
		{ID: "test_config", Title: "Test Configuration", Icon: "check-circle", Color: "secondary", RequiresConfirmation: false},
	}
}
func (m *Module) ExecuteAction(ctx context.Context, actionID string) error {
	switch actionID {
	case "reload":
		return m.deps.Agent.ServiceReload(ctx, "nginx")
	case "test_config":
		_, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil {
			return fmt.Errorf("nginx -t failed: %v\n%s", err, stderr)
		}
		m.log.Info("Config test passed", "output", stdout)
		return nil
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}
func (m *Module) Logs() []contract.LogDef {
	return []contract.LogDef{
		{ID: "service", Name: "Systemd Service Log", Type: "systemd"},
		{ID: "access", Name: "Global Access Log", Type: "file", Path: "/var/log/nginx/access.log"},
		{ID: "error", Name: "Global Error Log", Type: "file", Path: "/var/log/nginx/error.log"},
	}
}
func (m *Module) SettingsSchema() []contract.SettingField {
	return []contract.SettingField{
		{
			ID:          "worker_processes",
			Type:        "select",
			Label:       "Worker Processes",
			Description: "Number of worker processes (usually auto)",
			Value:       "auto",
			Options:     []string{"auto", "1", "2", "4", "8"},
			Category:    "Performance",
		},
		{
			ID:          "server_tokens",
			Type:        "boolean",
			Label:       "Server Tokens",
			Description: "Emit nginx version on error pages and in the 'Server' response header",
			Value:       false,
			Category:    "Security",
		},
	}
}

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
	
	// Nginx specific pages
	pages = append(pages, contract.ModulePage{ID: "sites", Title: "Virtual Hosts", Type: contract.PageTypeDataGrid})
	pages = append(pages, contract.ModulePage{ID: "certificates", Title: "Certificates", Type: contract.PageTypeDataGrid})
	
	return pages
}
