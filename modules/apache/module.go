// Package apache implements the Apache module for OpenDeploy.
package apache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "apache"

// Module is the Apache OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new Apache Module.
func New() *Module { return &Module{} }

// ─── contract.Module interface ─────────────────────────────────────────────

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Apache Web Server" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Powerful, flexible, HTTP/1.1 compliant web server" }

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

// Bootstrap stores the injected dependencies.
func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("apache module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }

func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{
			ID:    "apache",
			Label: "Apache",
			Icon:  "server",
			Path:  "/modules/apache",
			Order: 11,
		},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key:          "max_clients",
			Label:        "Max Clients",
			Description:  "Maximum number of simultaneous requests",
			Type:         contract.SettingTypeInt,
			DefaultValue: "150",
			Required:     true,
		},
	}
}

func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "apache: installing")
	ch, err := m.deps.Agent.PackageInstall(ctx, "apache2")
	if err != nil {
		return fmt.Errorf("apache: install: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	m.logger.InfoContext(ctx, "apache: uninstalling")
	ch, err := m.deps.Agent.PackageRemove(ctx, "apache2")
	if err != nil {
		return fmt.Errorf("apache: uninstall: %w", err)
	}
	for range ch {
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "apache2"); err != nil {
		return fmt.Errorf("apache: enable: %w", err)
	}
	return m.deps.Agent.ServiceStart(ctx, "apache2")
}

func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "apache2"); err != nil {
		return fmt.Errorf("apache: stop: %w", err)
	}
	return m.deps.Agent.ServiceDisable(ctx, "apache2")
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "apache2")
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "apache2")

	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "apache2")
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
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "apache2")
	if err != nil {
		return &contract.HealthReport{
			Status:  contract.HealthError,
			Message: fmt.Sprintf("cannot query apache service: %v", err),
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
	configPath := fmt.Sprintf("/etc/apache2/sites-available/opendeploy-%s.conf", site.PrimaryDomain)
	enabledPath := fmt.Sprintf("/etc/apache2/sites-enabled/opendeploy-%s.conf", site.PrimaryDomain)

	switch action {
	case contract.SiteUpsert, contract.SiteEnable:
		if action == contract.SiteUpsert {
			content := renderApache(site)
			if err := m.deps.Agent.FileWrite(ctx, configPath, content, 0o644); err != nil {
				return fmt.Errorf("apache: write config: %w", err)
			}
		}
		if err := m.deps.Agent.FileWrite(ctx, enabledPath, renderApache(site), 0o644); err != nil {
			return fmt.Errorf("apache: enable site: %w", err)
		}
	case contract.SiteDisable:
		_ = m.deps.Agent.FileDelete(ctx, enabledPath)
	case contract.SiteDelete:
		_ = m.deps.Agent.FileDelete(ctx, enabledPath)
		_ = m.deps.Agent.FileDelete(ctx, configPath)
	}

	return m.deps.Agent.ServiceRestart(ctx, "apache2")
}

func renderApache(site contract.SiteSpec) []byte {
	var b strings.Builder
	b.WriteString("# Managed by OpenDeploy Apache Module. Manual changes will be overwritten.\n")

	if !site.SSLEnabled {
		b.WriteString("<VirtualHost *:80>\n")
		fmt.Fprintf(&b, "    ServerName %s\n", site.PrimaryDomain)
		fmt.Fprintf(&b, "    DocumentRoot %s\n", site.RootPath)
		b.WriteString("\n    <Directory " + site.RootPath + ">\n")
		b.WriteString("        AllowOverride All\n")
		b.WriteString("        Require all granted\n")
		b.WriteString("    </Directory>\n")
		if site.AppVersion != "" {
			b.WriteString("\n    <FilesMatch \\.php$>\n")
			fmt.Fprintf(&b, "        SetHandler \"proxy:unix:/run/php/php%s-fpm.sock|fcgi://localhost\"\n", site.AppVersion)
			b.WriteString("    </FilesMatch>\n")
		}
		b.WriteString("</VirtualHost>\n")
	} else {
		b.WriteString("<VirtualHost *:443>\n")
		fmt.Fprintf(&b, "    ServerName %s\n", site.PrimaryDomain)
		fmt.Fprintf(&b, "    DocumentRoot %s\n", site.RootPath)
		b.WriteString("    SSLEngine on\n")
		fmt.Fprintf(&b, "    SSLCertificateFile %s\n", site.SSLCert)
		fmt.Fprintf(&b, "    SSLCertificateKeyFile %s\n", site.SSLKey)
		b.WriteString("\n    <Directory " + site.RootPath + ">\n")
		b.WriteString("        AllowOverride All\n")
		b.WriteString("        Require all granted\n")
		b.WriteString("    </Directory>\n")
		if site.AppVersion != "" {
			b.WriteString("\n    <FilesMatch \\.php$>\n")
			fmt.Fprintf(&b, "        SetHandler \"proxy:unix:/run/php/php%s-fpm.sock|fcgi://localhost\"\n", site.AppVersion)
			b.WriteString("    </FilesMatch>\n")
		}
		b.WriteString("</VirtualHost>\n")
	}

	return []byte(b.String())
}

func boolHealth(ok bool) contract.HealthStatus {
	if ok {
		return contract.HealthOK
	}
	return contract.HealthError
}

func formatServiceMsg(active bool) string {
	if active {
		return "apache service is running"
	}
	return "apache service is not running"
}

// compile-time assertion
var _ contract.WebServerPlugin = (*Module)(nil)

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
