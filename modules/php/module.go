// Package php implements the PHP module for OpenDeploy.
// Supports multiple PHP versions via php8.x-fpm packages.
package php

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "php"

// supportedVersions lists PHP versions that can be installed via this module.
var supportedVersions = []string{"8.1", "8.2", "8.3", "8.4"}

// Module is the PHP OpenDeploy module.
type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

// New creates a new PHP Module.
func New() *Module { return &Module{} }

func (m *Module) ID() string      { return moduleID }
func (m *Module) Name() string    { return "PHP" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Description() string {
	return "PHP scripting language with FPM support (multiple versions)"
}

func (m *Module) Category() string { return "Languages" }
func (m *Module) Icon() string     { return "code" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true, // fpm pools are services
		SupportsSettings: true,
		SupportsLogs:     true,
		SupportsRestart:  true,
		SupportsUpdate:   true,
	}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("php module bootstrapped")

	m.deps.Events.Subscribe("site.created", m.handleSiteEvent)
	m.deps.Events.Subscribe("site.updated", m.handleSiteEvent)
	m.deps.Events.Subscribe("site.deleted", m.handleSiteEvent)

	return nil
}

func (m *Module) handleSiteEvent(ctx context.Context, event contract.Event) error {
	b, err := json.Marshal(event.Payload())
	if err != nil {
		return nil // gracefully ignore invalid payloads
	}
	var data struct {
		AppType       string `json:"app_type"`
		AppVersion    string `json:"app_version"`
		PrimaryDomain string `json:"primary_domain"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil
	}

	if data.AppType != "php" || data.PrimaryDomain == "" {
		return nil
	}

	version := data.AppVersion
	if version == "" {
		version = m.deps.Config.Get("default_version", "8.3")
	}

	switch event.Type() {
	case "site.created", "site.updated":
		cfg := PoolConfig{
			Name: data.PrimaryDomain, User: "www-data", Group: "www-data",
			Listen: fmt.Sprintf("/run/php/php%s-fpm-%s.sock", version, data.PrimaryDomain),
			MaxChildren: 5, StartServers: 2, MinSpareServers: 1, MaxSpareServers: 3,
		}
		if err := m.CreatePool(ctx, version, cfg); err != nil {
			m.logger.ErrorContext(ctx, "failed to create php pool", "error", err, "domain", data.PrimaryDomain)
		}
	case "site.deleted":
		if err := m.DeletePool(ctx, version, data.PrimaryDomain); err != nil {
			m.logger.ErrorContext(ctx, "failed to delete php pool", "error", err, "domain", data.PrimaryDomain)
		}
	}
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }

func (m *Module) RegisterRoutes(_ contract.Router) {}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: "php", Label: "PHP", Icon: "code", Path: "/modules/php", Order: 20},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return []contract.SettingSpec{
		{
			Key: "default_version", Label: "Default PHP Version",
			Type: contract.SettingTypeSelect, Options: supportedVersions,
			DefaultValue: "8.3", Required: true,
		},
	}
}

// Install installs the default PHP version (8.3) with FPM.
func (m *Module) Install(ctx context.Context) error {
	return m.InstallVersion(ctx, "8.3")
}

// InstallVersion installs a specific PHP version.
func (m *Module) InstallVersion(ctx context.Context, version string) error {
	pkg := fmt.Sprintf("php%s-fpm", version)
	m.logger.InfoContext(ctx, "php: installing", "version", version, "package", pkg)
	ch, err := m.deps.Agent.PackageInstall(ctx, pkg)
	if err != nil {
		return fmt.Errorf("php: install %s: %w", version, err)
	}
	for range ch {
	}

	// Also install common extensions.
	common := fmt.Sprintf("php%s-cli php%s-mbstring php%s-xml php%s-curl php%s-zip", version, version, version, version, version)
	_ = common // install separately as needed
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	for _, v := range supportedVersions {
		svc := fmt.Sprintf("php%s-fpm", v)
		ch, err := m.deps.Agent.PackageRemove(ctx, svc)
		if err != nil {
			continue
		}
		for range ch {
		}
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	// Enable the default version.
	svc := "php8.3-fpm"
	if err := m.deps.Agent.ServiceEnable(ctx, svc); err != nil {
		return err
	}
	return m.deps.Agent.ServiceStart(ctx, svc)
}

func (m *Module) Disable(ctx context.Context) error {
	for _, v := range supportedVersions {
		svc := fmt.Sprintf("php%s-fpm", v)
		_ = m.deps.Agent.ServiceStop(ctx, svc)
		_ = m.deps.Agent.ServiceDisable(ctx, svc)
	}
	return nil
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "php8.3-fpm")
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "php8.3-fpm")
	pkgStatus := contract.PackageNotInstalled
	if installed {
		pkgStatus = contract.PackageInstalled
	}

	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "php8.3-fpm")
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
	checks := make([]contract.HealthCheck, 0, len(supportedVersions))
	anyRunning := false
	for _, v := range supportedVersions {
		svc := fmt.Sprintf("php%s-fpm", v)
		st, err := m.deps.Agent.ServiceStatus(ctx, svc)
		if err != nil {
			continue
		}
		if st.Active {
			anyRunning = true
		}
		status := contract.HealthOK
		if !st.Active {
			status = contract.HealthWarning
		}
		checks = append(checks, contract.HealthCheck{
			Name: fmt.Sprintf("php%s-fpm", v), Status: status,
		})
	}
	overall := contract.HealthOK
	if !anyRunning {
		overall = contract.HealthWarning
	}
	return &contract.HealthReport{Status: overall, Checks: checks}, nil
}

// SupportedVersions returns the list of PHP versions this module supports.
func (m *Module) SupportedVersions() []string { return supportedVersions }

var _ contract.Module = (*Module)(nil)

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
