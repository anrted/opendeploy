package fail2ban

import (
	"context"
	"fmt"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
)

type Module struct {
	agent contract.AgentClient
}

func New() *Module {
	return &Module{}
}

func (m *Module) ID() string      { return "fail2ban" }
func (m *Module) Name() string    { return "Fail2Ban" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Description() string {
	return "Intrusion prevention software framework that protects computer servers from brute-force attacks"
}
func (m *Module) Category() string { return "Security" }
func (m *Module) Icon() string     { return "shield" }

func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}

func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true,
		SupportsSettings: true,
		SupportsLogs:     true,
	}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.agent = deps.Agent
	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) RegisterRoutes(r contract.Router)         {}
func (m *Module) RegisterMenuItems() []contract.MenuItem   { return nil }
func (m *Module) RegisterSettings() []contract.SettingSpec { return nil }

func (m *Module) Install(ctx context.Context) error {
	out, err := m.agent.PackageInstall(ctx, "fail2ban")
	if err != nil {
		return err
	}
	for range out {
	} // consume channel
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	out, err := m.agent.PackageRemove(ctx, "fail2ban")
	if err != nil {
		return err
	}
	for range out {
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	return m.agent.ServiceEnable(ctx, "fail2ban")
}

func (m *Module) Disable(ctx context.Context) error {
	return m.agent.ServiceDisable(ctx, "fail2ban")
}

func (m *Module) Restart(ctx context.Context) error {
	return m.agent.ServiceRestart(ctx, "fail2ban")
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	status, err := m.agent.ServiceStatus(ctx, "fail2ban")
	if err != nil {
		return nil, err
	}
	_, stdout, stderr, _ := m.agent.CommandExecute(ctx, "fail2ban-client", "--version")
	versionOutput := stdout
	if strings.TrimSpace(versionOutput) == "" {
		versionOutput = stderr
	}

	pkgStatus := contract.PackageNotInstalled
	if status.Active || status.Enabled {
		pkgStatus = contract.PackageInstalled
	}

	srvState := contract.ServiceStopped
	if status.Active {
		srvState = contract.ServiceRunning
	}

	return &contract.RuntimeStatus{
		PackageStatus:   pkgStatus,
		ServiceStatus:   srvState,
		SoftwareVersion: normalizeFail2BanVersion(versionOutput),
	}, nil
}

func normalizeFail2BanVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if strings.EqualFold(field, "fail2ban") && i+1 < len(fields) {
				return "Fail2Ban " + fields[i+1]
			}
		}
	}

	version := strings.TrimSpace(output)
	if version == "" {
		return "unknown"
	}
	return version
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	st, err := m.Status(ctx)
	if err != nil {
		return &contract.HealthReport{Status: contract.HealthError, Message: err.Error()}, nil
	}
	if st.ServiceStatus != contract.ServiceRunning {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "Fail2Ban is not active"}, nil
	}
	return &contract.HealthReport{Status: contract.HealthOK, Message: "Fail2Ban is running perfectly"}, nil
}

// ─── Metadata-driven UI ───────────────────────────────────────────────────

func (m *Module) Pages() []contract.ModulePage {
	return []contract.ModulePage{
		{ID: "overview", Title: "Overview", Type: contract.PageTypeOverview},
		{ID: "jails", Title: "Jails", Type: contract.PageTypeDataGrid},
		{ID: "banned_ips", Title: "Banned IP", Type: contract.PageTypeDataGrid},
		{ID: "logs", Title: "Logs", Type: contract.PageTypeLogs},
		{ID: "settings", Title: "Settings", Type: contract.PageTypeSettings},
	}
}

func (m *Module) Actions() []contract.ActionDef {
	return []contract.ActionDef{
		{
			ID:                   "enable_preset_sshd",
			Title:                "Enable SSH Protection",
			Description:          "Ban an IP for 24 hours after 5 failed SSH logins in 10 minutes.",
			Icon:                 "shield",
			Color:                "success",
			RequiresConfirmation: true,
		},
		{
			ID:                   "enable_preset_nginx_scanners",
			Title:                "Enable Nginx Scanner Protection",
			Description:          "Ban repeated directory scans and oversized request probes.",
			Icon:                 "shield",
			Color:                "success",
			RequiresConfirmation: true,
		},
		{
			ID:                   "enable_preset_nginx_auth",
			Title:                "Enable Nginx Auth Protection",
			Description:          "Ban repeated failed Nginx HTTP authentication attempts.",
			Icon:                 "shield",
			Color:                "success",
			RequiresConfirmation: true,
		},
		{
			ID:                   "enable_preset_php_probes",
			Title:                "Enable PHP Probe Protection",
			Description:          "Ban repeated probes for .env, phpinfo, WordPress, phpMyAdmin, PHPUnit and CGI paths.",
			Icon:                 "shield",
			Color:                "success",
			RequiresConfirmation: true,
		},
		{
			ID:                   "disable_preset_sshd",
			Title:                "Disable SSH Protection",
			Description:          "Remove the OpenDeploy SSH jail.",
			Icon:                 "shield-off",
			Color:                "warning",
			RequiresConfirmation: true,
		},
		{
			ID:                   "disable_preset_nginx_scanners",
			Title:                "Disable Nginx Scanner Protection",
			Description:          "Remove the OpenDeploy Nginx scanner jail.",
			Icon:                 "shield-off",
			Color:                "warning",
			RequiresConfirmation: true,
		},
		{
			ID:                   "disable_preset_nginx_auth",
			Title:                "Disable Nginx Auth Protection",
			Description:          "Remove the OpenDeploy Nginx authentication jail.",
			Icon:                 "shield-off",
			Color:                "warning",
			RequiresConfirmation: true,
		},
		{
			ID:                   "disable_preset_php_probes",
			Title:                "Disable PHP Probe Protection",
			Description:          "Remove the OpenDeploy PHP probe jail.",
			Icon:                 "shield-off",
			Color:                "warning",
			RequiresConfirmation: true,
		},
		{ID: "reload", Title: "Reload", Icon: "refresh-cw", Color: "secondary"},
		{ID: "restart", Title: "Restart", Icon: "rotate-cw", Color: "warning", Dangerous: true, RequiresConfirmation: true},
	}
}

func (m *Module) ExecuteAction(ctx context.Context, actionID string) error {
	switch actionID {
	case "enable_preset_sshd":
		return m.enableProtectionPreset(ctx, "sshd")
	case "enable_preset_nginx_scanners":
		return m.enableProtectionPreset(ctx, "nginx_scanners")
	case "enable_preset_nginx_auth":
		return m.enableProtectionPreset(ctx, "nginx_auth")
	case "enable_preset_php_probes":
		return m.enableProtectionPreset(ctx, "php_probes")
	case "disable_preset_sshd":
		return m.disableProtectionPreset(ctx, "sshd")
	case "disable_preset_nginx_scanners":
		return m.disableProtectionPreset(ctx, "nginx_scanners")
	case "disable_preset_nginx_auth":
		return m.disableProtectionPreset(ctx, "nginx_auth")
	case "disable_preset_php_probes":
		return m.disableProtectionPreset(ctx, "php_probes")
	case "reload":
		_, _, _, err := m.agent.CommandExecute(ctx, "fail2ban-client", "reload")
		return err
	case "restart":
		return m.Restart(ctx)
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}

func (m *Module) Logs() []contract.LogDef {
	return []contract.LogDef{
		{ID: "systemd", Name: "Service Logs (journald)", Type: "systemd", Path: "fail2ban"},
		{ID: "fail2ban_log", Name: "Fail2Ban Log", Type: "file", Path: "/var/log/fail2ban.log"},
	}
}

func (m *Module) SettingsSchema() []contract.SettingField {
	return []contract.SettingField{
		// GENERAL
		{ID: "enabled", Label: "Enabled", Category: "General", Type: "boolean", Advanced: false, RequiresRestart: false},
		{ID: "backend", Label: "Backend", Category: "General", Type: "select", Options: []string{"auto", "systemd", "polling"}, Advanced: true, RequiresRestart: true},
		{ID: "loglevel", Label: "Log Level", Category: "General", Type: "select", Options: []string{"INFO", "DEBUG", "WARN", "ERROR"}, Advanced: false, RequiresRestart: false},
		{ID: "logtarget", Label: "Log Target", Category: "General", Type: "string", Advanced: true, RequiresRestart: true},
		{ID: "allowipv6", Label: "Allow IPv6", Category: "General", Type: "select", Options: []string{"auto", "yes", "no"}, Advanced: true, RequiresRestart: true},
		{ID: "socket", Label: "Socket", Category: "General", Type: "string", Advanced: true, RequiresRestart: true},
		{ID: "pidfile", Label: "PID File", Category: "General", Type: "string", Advanced: true, RequiresRestart: true},
		{ID: "dbfile", Label: "Database File", Category: "General", Type: "string", Advanced: true, RequiresRestart: true},
		{ID: "dbpurgeage", Label: "DB Purge Age", Category: "General", Type: "string", Advanced: true, RequiresRestart: false, ValidationRegex: "^[0-9]+[smhdwy]?$"},

		// BAN SETTINGS
		{ID: "bantime", Label: "Ban Time", Category: "Ban Settings", Type: "string", Advanced: false, RequiresRestart: false, ValidationRegex: "^[0-9]+[smhdwy]?$"},
		{ID: "findtime", Label: "Find Time", Category: "Ban Settings", Type: "string", Advanced: false, RequiresRestart: false, ValidationRegex: "^[0-9]+[smhdwy]?$"},
		{ID: "maxretry", Label: "Max Retry", Category: "Ban Settings", Type: "number", Advanced: false, RequiresRestart: false},
		{ID: "banaction", Label: "Ban Action", Category: "Ban Settings", Type: "string", Advanced: true, RequiresRestart: false, Placeholder: "e.g. iptables-multiport"},
		{ID: "banaction_allports", Label: "Ban Action All Ports", Category: "Ban Settings", Type: "string", Advanced: true, RequiresRestart: false},
		{ID: "ignoreip", Label: "Ignore IP List", Category: "Ban Settings", Type: "string", Advanced: false, RequiresRestart: false, Placeholder: "127.0.0.1/8 ::1"},
		{ID: "bantime.increment", Label: "Incremental Ban", Category: "Ban Settings", Type: "boolean", Advanced: true, RequiresRestart: false},
		{ID: "bantime.maxtime", Label: "Maximum Ban Time", Category: "Ban Settings", Type: "string", Advanced: true, RequiresRestart: false, ValidationRegex: "^[0-9]+[smhdwy]?$"},
		{ID: "bantime.random", Label: "Randomize Ban Time", Category: "Ban Settings", Type: "boolean", Advanced: true, RequiresRestart: false},

		// NOTIFICATIONS
		{ID: "destemail", Label: "Destination Email", Category: "Notifications", Type: "string", Advanced: false, RequiresRestart: false, ValidationRegex: "^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\\.[a-zA-Z0-9-.]+$"},
		{ID: "sender", Label: "Sender Email", Category: "Notifications", Type: "string", Advanced: true, RequiresRestart: false},
		{ID: "sendername", Label: "Sender Name", Category: "Notifications", Type: "string", Advanced: true, RequiresRestart: false},
		{ID: "mta", Label: "Mail Transport", Category: "Notifications", Type: "select", Options: []string{"sendmail", "mail"}, Advanced: true, RequiresRestart: false},
		{ID: "action", Label: "Action on Ban", Category: "Notifications", Type: "select", Options: []string{"action_", "action_mw", "action_mwl"}, Advanced: true, RequiresRestart: false},

		// SECURITY
		{ID: "usedns", Label: "DNS Resolution", Category: "Security", Type: "select", Options: []string{"warn", "yes", "no", "raw"}, Advanced: true, RequiresRestart: false},
	}
}

// ─── DataGridProvider Implementation ──────────────────────────────────────

func (m *Module) DataGridSchema(pageID string) (contract.DataGridSchema, error) {
	switch pageID {
	case "jails":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "name", Title: "Jail Name", Type: "text", Sortable: true},
				{Key: "status", Title: "Status", Type: "badge", Sortable: true},
				{Key: "banned", Title: "Banned IPs", Type: "number", Sortable: true},
			},
			Actions: []contract.ActionDef{
				{ID: "unban_all", Title: "Unban All", Icon: "unlock", Dangerous: true},
			},
		}, nil
	case "banned_ips":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "ip", Title: "IP Address", Type: "text", Sortable: true},
				{Key: "jail", Title: "Jail", Type: "badge", Sortable: true},
			},
			Actions: []contract.ActionDef{
				{ID: "unban", Title: "Unban", Icon: "unlock"},
			},
		}, nil
	default:
		return contract.DataGridSchema{}, fmt.Errorf("unknown datagrid page: %s", pageID)
	}
}

func (m *Module) DataGridData(ctx context.Context, pageID string) ([]map[string]any, error) {
	if pageID == "jails" {
		// Example data for now
		return []map[string]any{
			{"name": "sshd", "status": "Active", "banned": 12},
			{"name": "nginx-http-auth", "status": "Active", "banned": 0},
		}, nil
	}
	if pageID == "banned_ips" {
		// Example data
		return []map[string]any{
			{"ip": "192.168.1.100", "jail": "sshd"},
			{"ip": "10.0.0.5", "jail": "nginx-http-auth"},
		}, nil
	}
	return nil, fmt.Errorf("unknown datagrid page: %s", pageID)
}

func (m *Module) DataGridAction(ctx context.Context, pageID string, actionID string, payload map[string]any) error {
	return nil
}
