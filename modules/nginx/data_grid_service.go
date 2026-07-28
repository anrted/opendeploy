package nginx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

// DataGridService owns the dynamic Nginx administration pages.
type DataGridService struct {
	module *Module
	agent  contract.AgentClient
}

func NewDataGridService(module *Module, agent contract.AgentClient) *DataGridService {
	return &DataGridService{module: module, agent: agent}
}

func (s *DataGridService) Pages(capabilities contract.ModuleCapabilities) []contract.ModulePage {
	pages := []contract.ModulePage{{ID: "overview", Title: "Overview", Type: contract.PageTypeOverview}}
	if capabilities.SupportsSettings {
		pages = append(pages, contract.ModulePage{ID: "settings", Title: "Settings", Type: contract.PageTypeSettings})
	}
	if capabilities.SupportsLogs {
		pages = append(pages, contract.ModulePage{ID: "logs", Title: "Logs", Type: contract.PageTypeLogs})
	}
	return append(pages,
		contract.ModulePage{ID: "sites", Title: "Virtual Hosts", Type: contract.PageTypeDataGrid},
		contract.ModulePage{ID: "certificates", Title: "Certificates", Type: contract.PageTypeDataGrid},
		contract.ModulePage{ID: "configuration", Title: "Configuration", Type: contract.PageTypeDataGrid},
	)
}

func (s *DataGridService) Schema(pageID string) (contract.DataGridSchema, error) {
	switch pageID {
	case "sites":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "domain", Title: "Domain", Type: "text"}, {Key: "port", Title: "Port", Type: "text"},
				{Key: "root", Title: "Root Directory", Type: "text"}, {Key: "php", Title: "PHP", Type: "text"},
				{Key: "ssl", Title: "SSL", Type: "badge"}, {Key: "status", Title: "Status", Type: "badge"},
				{Key: "modified", Title: "Last Modified", Type: "date"},
			},
			RowActions: []contract.ActionDef{
				{ID: "enable", Title: "Enable", Icon: "play", Color: "success"},
				{ID: "disable", Title: "Disable", Icon: "square", Color: "warning", RequiresConfirmation: true},
				{ID: "delete", Title: "Delete", Icon: "trash-2", Color: "danger", RequiresConfirmation: true},
			},
		}, nil
	case "certificates":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "domain", Title: "Domain", Type: "text"}, {Key: "provider", Title: "Provider", Type: "badge"},
				{Key: "issuer", Title: "Issuer", Type: "text"}, {Key: "issued_at", Title: "Issued", Type: "text"},
				{Key: "expires", Title: "Expires", Type: "text"}, {Key: "remaining", Title: "Remaining", Type: "text"},
				{Key: "san", Title: "SAN", Type: "text"}, {Key: "path", Title: "Certificate Path", Type: "text"},
				{Key: "status", Title: "Status", Type: "badge"},
			},
			RowActions: []contract.ActionDef{{ID: "renew", Title: "Renew", Icon: "refresh-cw", Color: "primary"}},
		}, nil
	case "configuration":
		return contract.DataGridSchema{
			Columns: []contract.DataGridColumn{
				{Key: "path", Title: "Path", Type: "text"}, {Key: "kind", Title: "Type", Type: "badge"},
				{Key: "modified", Title: "Last Modified", Type: "date"}, {Key: "preview", Title: "Preview", Type: "text"},
			},
			Actions: []contract.ActionDef{
				{ID: "validate_config", Title: "Validate Configuration", Icon: "check-circle", Color: "primary"},
				{ID: "reload_config", Title: "Reload Configuration", Icon: "refresh-cw", Color: "primary"},
			},
			RowActions: []contract.ActionDef{{
				ID: "save_config", Title: "Edit Configuration", Icon: "edit", Color: "warning", RequiresConfirmation: true,
				Inputs: []contract.ActionInputDef{{Key: "content", Label: "Configuration content", Type: "textarea", Required: true}},
			}},
		}, nil
	default:
		return contract.DataGridSchema{}, fmt.Errorf("unknown page id: %s", pageID)
	}
}

func (s *DataGridService) Data(ctx context.Context, pageID string) ([]map[string]any, error) {
	switch pageID {
	case "sites":
		return s.siteRows(ctx)
	case "certificates":
		return s.module.certificateRows(ctx)
	case "configuration":
		return s.module.configurationRows(ctx)
	default:
		return nil, fmt.Errorf("unknown page id: %s", pageID)
	}
}

func (s *DataGridService) siteRows(ctx context.Context) ([]map[string]any, error) {
	var sites []map[string]any
	entries, err := s.agent.DirList(ctx, "/etc/nginx/sites-available")
	if err != nil {
		return sites, nil
	}
	for _, entry := range entries {
		if entry.IsDir || !strings.HasPrefix(entry.Name, "opendeploy-") || !strings.HasSuffix(entry.Name, ".conf") {
			continue
		}
		domain := strings.TrimSuffix(strings.TrimPrefix(entry.Name, "opendeploy-"), ".conf")
		status := "disabled"
		if _, readErr := s.agent.FileRead(ctx, fmt.Sprintf("/etc/nginx/sites-enabled/opendeploy-%s.conf", domain)); readErr == nil {
			status = "enabled"
		}
		content, _ := s.agent.FileRead(ctx, entry.Path)
		config := string(content)
		port := strings.Fields(nginxDirective(config, "listen"))
		listenPort := ""
		if len(port) > 0 {
			listenPort = port[0]
		}
		sites = append(sites, map[string]any{
			"domain": domain, "port": listenPort, "root": nginxDirective(config, "root"),
			"php": nginxPHPVersion(config), "ssl": nginxSSLStatus(config), "status": status,
			"modified": entry.ModTime.UTC().Format(time.RFC3339),
		})
	}
	return sites, nil
}

func (s *DataGridService) Action(ctx context.Context, pageID, actionID string, payload map[string]any) error {
	if pageID == "sites" {
		domain, _ := payload["domain"].(string)
		if domain == "" {
			return fmt.Errorf("domain is required")
		}
		spec := contract.SiteSpec{PrimaryDomain: domain}
		switch actionID {
		case "enable":
			return s.module.ApplySite(ctx, contract.SiteEnable, spec)
		case "disable":
			return s.module.ApplySite(ctx, contract.SiteDisable, spec)
		case "delete":
			return s.module.ApplySite(ctx, contract.SiteDelete, spec)
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
		exitCode, _, stderr, err := s.agent.CommandExecute(ctx, "certbot", "renew", "--cert-name", domain, "--non-interactive")
		if err != nil || exitCode != 0 {
			return fmt.Errorf("renew certificate for %s: %s", domain, strings.TrimSpace(stderr))
		}
		return s.agent.ServiceReload(ctx, "nginx")
	}
	if pageID == "configuration" {
		switch actionID {
		case "validate_config":
			exitCode, stdout, stderr, err := s.agent.CommandExecute(ctx, "nginx", "-t")
			if err != nil || exitCode != 0 {
				return fmt.Errorf("nginx configuration is invalid: %s %s", strings.TrimSpace(stdout), strings.TrimSpace(stderr))
			}
			return nil
		case "reload_config":
			return s.module.validateAndReload(ctx)
		case "save_config":
			path, _ := payload["path"].(string)
			content, _ := payload["content"].(string)
			return s.module.saveConfigurationFile(ctx, path, content)
		}
	}
	return fmt.Errorf("unknown page or action")
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
