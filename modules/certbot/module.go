// Package certbot implements the Certbot module for OpenDeploy.
package certbot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
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

func (m *Module) Category() string { return "Security" }
func (m *Module) Icon() string     { return "shield" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true, // certbot.timer
		SupportsSettings: false,
		SupportsLogs:     true,
		SupportsRestart:  true,
		SupportsUpdate:   true,
	}
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
			Required:    true,
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

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "certbot")
	pkgStatus := contract.PackageNotInstalled
	if installed {
		pkgStatus = contract.PackageInstalled
	}

	svcStatus, err := m.deps.Agent.ServiceStatus(ctx, "certbot.timer")
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

// ─── CertbotPlugin ─────────────────────────────────────────────────────────

func (m *Module) ObtainCert(ctx context.Context, domain, webroot string) error {
	installed, _, _ := m.deps.Agent.PackageInstalled(ctx, "certbot")
	if !installed {
		return apperrors.InvalidInput("Certbot is not installed on the server. Please install the Certbot module first.")
	}

	// Ensure the webroot exists and is readable by the web server
	if err := m.deps.Agent.DirCreate(ctx, webroot, 0o755); err != nil {
		m.logger.WarnContext(ctx, "failed to create webroot for certbot", "webroot", webroot, "error", err)
	}

	email := m.deps.Config.Get("email", "")
	if typed, ok := m.deps.Agent.(contract.SiteRuntimeAgentClient); ok && !servercontext.IsLocal(ctx) {
		if err := typed.CertificateObtain(ctx, domain, webroot, email); err != nil {
			m.logger.WarnContext(ctx, "certbot obtain failed", "domain", domain)
			return apperrors.InvalidInput("Certbot failed to obtain the certificate; verify DNS and port 80 reachability")
		}
		return nil
	}
	args := []string{"certonly", "--webroot", "-w", webroot, "-d", domain, "--agree-tos", "-n"}
	if email == "" {
		args = append(args, "--register-unsafely-without-email")
	} else {
		args = append(args, "-m", email)
	}
	exitCode, _, stderr, err := m.deps.Agent.CommandExecute(ctx, "certbot", args...)
	if err != nil || exitCode != 0 {
		m.logger.WarnContext(ctx, "certbot obtain failed", "domain", domain, "exit_code", exitCode)
		return apperrors.InvalidInput("Certbot failed to obtain the certificate; verify DNS and port 80 reachability")
	}
	_ = stderr // Agent-side details are logged and are not reflected to clients.
	return nil
}

var _ contract.CertbotPlugin = (*Module)(nil)

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
