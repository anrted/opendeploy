package nginx

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

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
		SupportsSettings: true,
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

// compile-time assertion
var _ contract.WebServerPlugin = (*Module)(nil)

func (m *Module) Logs() []contract.LogDef {
	logs := []contract.LogDef{
		{ID: "service", Name: "Systemd Service Log", Type: "systemd"},
		{ID: "access", Name: "Global Access Log", Type: "file", Path: "/var/log/nginx/access.log"},
		{ID: "error", Name: "Global Error Log", Type: "file", Path: "/var/log/nginx/error.log"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	entries, err := m.deps.Agent.DirList(ctx, "/var/log/nginx")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir || !strings.HasSuffix(entry.Name, ".log") || entry.Name == "access.log" || entry.Name == "error.log" {
				continue
			}
			logID := strings.TrimSuffix(entry.Name, ".log")
			logs = append(logs, contract.LogDef{
				ID:   logID,
				Name: entry.Name,
				Type: "file",
				Path: entry.Path,
			})
		}
	}
	return logs
}

func (m *Module) ReadLog(ctx context.Context, logID string, lines int) ([]string, error) {
	if logID == "service" {
		return m.deps.Agent.ServiceLogs(ctx, "nginx", lines)
	}

	var path string
	for _, l := range m.Logs() {
		if l.ID == logID && l.Type == "file" {
			path = l.Path
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("log %s not found", logID)
	}

	return m.deps.Agent.FileLogs(ctx, path, lines)
}

func (m *Module) ClearLog(ctx context.Context, logID string) error {
	if logID == "service" {
		return fmt.Errorf("cannot clear systemd service logs directly")
	}

	var path string
	for _, l := range m.Logs() {
		if l.ID == logID && l.Type == "file" {
			path = l.Path
			break
		}
	}
	if path == "" {
		return fmt.Errorf("log %s not found", logID)
	}

	return m.deps.Agent.FileWrite(ctx, path, nil, 0o640)
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
	pages = append(pages, contract.ModulePage{ID: "configuration", Title: "Configuration", Type: contract.PageTypeDataGrid})

	return pages
}

func (m *Module) DataGridSchema(pageID string) (contract.DataGridSchema, error) {
	if pageID == "sites" {
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "domain", Title: "Domain", Type: "text"},
				{Key: "port", Title: "Port", Type: "text"},
				{Key: "root", Title: "Root Directory", Type: "text"},
				{Key: "php", Title: "PHP", Type: "text"},
				{Key: "ssl", Title: "SSL", Type: "badge"},
				{Key: "status", Title: "Status", Type: "badge"},
				{Key: "modified", Title: "Last Modified", Type: "date"},
			},
			RowActions: []contract.ActionDef{
				{ID: "enable", Title: "Enable", Icon: "play", Color: "success", RequiresConfirmation: false},
				{ID: "disable", Title: "Disable", Icon: "square", Color: "warning", RequiresConfirmation: true},
				{ID: "delete", Title: "Delete", Icon: "trash-2", Color: "danger", RequiresConfirmation: true},
			},
		}, nil
	}
	if pageID == "certificates" {
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "domain", Title: "Domain", Type: "text"},
				{Key: "provider", Title: "Provider", Type: "badge"},
				{Key: "issuer", Title: "Issuer", Type: "text"},
				{Key: "issued_at", Title: "Issued", Type: "text"},
				{Key: "expires", Title: "Expires", Type: "text"},
				{Key: "remaining", Title: "Remaining", Type: "text"},
				{Key: "san", Title: "SAN", Type: "text"},
				{Key: "path", Title: "Certificate Path", Type: "text"},
				{Key: "status", Title: "Status", Type: "badge"},
			},
			RowActions: []contract.ActionDef{
				{ID: "renew", Title: "Renew", Icon: "refresh-cw", Color: "primary"},
			},
		}, nil
	}
	if pageID == "configuration" {
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "path", Title: "Path", Type: "text"},
				{Key: "kind", Title: "Type", Type: "badge"},
				{Key: "modified", Title: "Last Modified", Type: "date"},
				{Key: "preview", Title: "Preview", Type: "text"},
			},
			Actions: []contract.ActionDef{
				{ID: "validate_config", Title: "Validate Configuration", Icon: "check-circle", Color: "primary"},
				{ID: "reload_config", Title: "Reload Configuration", Icon: "refresh-cw", Color: "primary"},
			},
			RowActions: []contract.ActionDef{
				{
					ID: "save_config", Title: "Edit Configuration", Icon: "edit", Color: "warning", RequiresConfirmation: true,
					Inputs: []contract.ActionInputDef{
						{Key: "content", Label: "Configuration content", Type: "textarea", Required: true},
					},
				},
			},
		}, nil
	}
	return contract.DataGridSchema{}, fmt.Errorf("unknown page id: %s", pageID)
}

func (m *Module) DataGridData(ctx context.Context, pageID string) ([]map[string]any, error) {
	if pageID == "sites" {
		var sites []map[string]any
		entries, err := m.deps.Agent.DirList(ctx, "/etc/nginx/sites-available")
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir || !strings.HasPrefix(entry.Name, "opendeploy-") || !strings.HasSuffix(entry.Name, ".conf") {
					continue
				}
				// Extract domain from filename
				domain := strings.TrimSuffix(strings.TrimPrefix(entry.Name, "opendeploy-"), ".conf")

				// Check status
				enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/opendeploy-%s.conf", domain)
				status := "disabled"
				if _, readErr := m.deps.Agent.FileRead(ctx, enabledPath); readErr == nil {
					status = "enabled"
				}
				content, _ := m.deps.Agent.FileRead(ctx, entry.Path)
				config := string(content)
				root := nginxDirective(config, "root")
				port := strings.Fields(nginxDirective(config, "listen"))
				listenPort := ""
				if len(port) > 0 {
					listenPort = port[0]
				}
				sites = append(sites, map[string]any{
					"domain":   domain,
					"port":     listenPort,
					"root":     root,
					"php":      nginxPHPVersion(config),
					"ssl":      nginxSSLStatus(config),
					"status":   status,
					"modified": entry.ModTime.UTC().Format(time.RFC3339),
				})
			}
		}
		return sites, nil
	}
	if pageID == "certificates" {
		return m.certificateRows(ctx)
	}
	if pageID == "configuration" {
		return m.configurationRows(ctx)
	}
	return nil, fmt.Errorf("unknown page id: %s", pageID)
}

func nginxPHPVersion(content string) string {
	value := nginxDirective(content, "fastcgi_pass")
	value = strings.TrimPrefix(value, "unix:/run/php/php")
	if value == "" || value == nginxDirective(content, "fastcgi_pass") {
		return ""
	}
	if index := strings.Index(value, "-fpm-"); index >= 0 {
		return value[:index]
	}
	return ""
}

func nginxSSLStatus(content string) string {
	if nginxDirective(content, "ssl_certificate") != "" {
		return "enabled"
	}
	return "disabled"
}

func nginxDirective(content, directive string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, directive+" ") {
			return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, directive)), ";")
		}
	}
	return ""
}

func (m *Module) DataGridAction(ctx context.Context, pageID, actionID string, payload map[string]any) error {
	if pageID == "sites" {
		domain, _ := payload["domain"].(string)
		if domain == "" {
			return fmt.Errorf("domain is required")
		}

		spec := contract.SiteSpec{PrimaryDomain: domain}

		switch actionID {
		case "enable":
			return m.ApplySite(ctx, contract.SiteEnable, spec)
		case "disable":
			return m.ApplySite(ctx, contract.SiteDisable, spec)
		case "delete":
			return m.ApplySite(ctx, contract.SiteDelete, spec)
		default:
			return fmt.Errorf("unknown action: %s", actionID)
		}
	}
	if pageID == "certificates" && actionID == "renew" {
		domain, _ := payload["domain"].(string)
		certificatePath, _ := payload["path"].(string)
		if !validManagedDomain(domain) {
			return fmt.Errorf("valid certificate domain is required")
		}
		if !strings.HasPrefix(certificatePath, "/etc/letsencrypt/") {
			return fmt.Errorf("only Let's Encrypt certificates can be renewed with Certbot")
		}
		exitCode, _, stderr, err := m.deps.Agent.CommandExecute(
			ctx, "certbot", "renew", "--cert-name", domain, "--non-interactive",
		)
		if err != nil || exitCode != 0 {
			return fmt.Errorf("renew certificate for %s: %s", domain, strings.TrimSpace(stderr))
		}
		return m.deps.Agent.ServiceReload(ctx, "nginx")
	}
	if pageID == "configuration" {
		switch actionID {
		case "validate_config":
			exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
			if err != nil || exitCode != 0 {
				return fmt.Errorf("nginx configuration is invalid: %s %s", strings.TrimSpace(stdout), strings.TrimSpace(stderr))
			}
			return nil
		case "reload_config":
			return m.validateAndReload(ctx)
		case "save_config":
			path, _ := payload["path"].(string)
			content, _ := payload["content"].(string)
			return m.saveConfigurationFile(ctx, path, content)
		}
	}
	return fmt.Errorf("unknown page or action")
}

func validManagedDomain(domain string) bool {
	if domain == "" || len(domain) > 253 || strings.ContainsAny(domain, `/\:@`) {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

var _ contract.DataGridProvider = (*Module)(nil)
