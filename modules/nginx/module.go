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
