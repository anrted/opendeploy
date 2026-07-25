// Package nginx provides typed, transactional Nginx configuration operations.
package nginx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/anrted/opendeploy/internal/agent/executor"
)

const (
	defaultAvailableDir = "/etc/nginx/sites-available"
	defaultEnabledDir   = "/etc/nginx/sites-enabled"
)

type Action string

const (
	ActionUpsert  Action = "upsert"
	ActionDelete  Action = "delete"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
)

type Site struct {
	Domain     string
	RootPath   string
	PHPVersion string
	SSLEnabled bool
	SSLCert    string
	SSLKey     string
}

type fileStore interface {
	Read(path string) ([]byte, error)
	Write(path string, content []byte, mode fs.FileMode) error
	Delete(path string) error
}

type commandRunner interface {
	Run(ctx context.Context, binary string, args ...string) (*executor.Result, error)
}

type linkStore interface {
	Exists(path string) (bool, error)
	Symlink(target, link string) error
	Remove(path string) error
}

type osLinkStore struct{}

func (osLinkStore) Exists(name string) (bool, error) {
	_, err := os.Lstat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (osLinkStore) Symlink(target, link string) error {
	if err := os.MkdirAll(path.Dir(link), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}

func (osLinkStore) Remove(name string) error {
	err := os.Remove(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

type Manager struct {
	files        fileStore
	runner       commandRunner
	links        linkStore
	availableDir string
	enabledDir   string
}

func NewManager(files fileStore, runner commandRunner) *Manager {
	return &Manager{
		files:        files,
		runner:       runner,
		links:        osLinkStore{},
		availableDir: defaultAvailableDir,
		enabledDir:   defaultEnabledDir,
	}
}

// Apply mutates one managed vhost, validates the complete Nginx configuration,
// reloads Nginx, and restores the previous state if either step fails.
// If SSLEnabled is true and certificates do not exist, it automatically provisions
// them via Certbot before applying the SSL configuration.
func (m *Manager) Apply(ctx context.Context, action Action, site Site) error {
	if err := validate(action, site); err != nil {
		return err
	}

	configPath, enabledPath := m.paths(site.Domain)
	before, err := m.snapshot(configPath, enabledPath)
	if err != nil {
		return fmt.Errorf("nginx: capture current configuration: %w", err)
	}

	needCertbot := false
	if action == ActionUpsert && site.SSLEnabled {
		_, err1 := os.Stat(site.SSLCert)
		_, err2 := os.Stat(site.SSLKey)
		if err1 != nil || err2 != nil {
			needCertbot = true
		}
	}

	if needCertbot {
		// Step 1: Apply HTTP-only configuration to serve the ACME challenge.
		httpSite := site
		httpSite.SSLEnabled = false
		if err := m.mutate(action, httpSite, configPath, enabledPath); err != nil {
			_ = m.restore(configPath, enabledPath, before)
			return fmt.Errorf("nginx: apply http config for certbot: %w", err)
		}
		if _, err := m.runner.Run(ctx, "nginx", "-t"); err != nil {
			_ = m.rollback(ctx, configPath, enabledPath, before)
			return fmt.Errorf("nginx: validation failed for http config: %w", err)
		}
		if _, err := m.runner.Run(ctx, "nginx", "-s", "reload"); err != nil {
			_ = m.rollback(ctx, configPath, enabledPath, before)
			return fmt.Errorf("nginx: reload failed for http config: %w", err)
		}

		// Step 2: Run Certbot to acquire the certificate.
		res, err := m.runner.Run(ctx, "certbot", "certonly", "--webroot", "-w", site.RootPath, "-d", site.Domain, "-n", "--agree-tos", "--register-unsafely-without-email", "--expand")
		if err != nil {
			_ = m.rollback(ctx, configPath, enabledPath, before)
			return fmt.Errorf("nginx: certbot failed: %s: %w", res.Stderr, err)
		}
	}

	// Step 3: Apply target configuration (HTTPS if requested).
	if err := m.mutate(action, site, configPath, enabledPath); err != nil {
		_ = m.restore(configPath, enabledPath, before)
		return fmt.Errorf("nginx: apply file changes: %w", err)
	}
	if _, err := m.runner.Run(ctx, "nginx", "-t"); err != nil {
		rollbackErr := m.rollback(ctx, configPath, enabledPath, before)
		return errors.Join(fmt.Errorf("nginx: configuration validation failed: %w", err), rollbackErr)
	}
	if _, err := m.runner.Run(ctx, "nginx", "-s", "reload"); err != nil {
		rollbackErr := m.rollback(ctx, configPath, enabledPath, before)
		return errors.Join(fmt.Errorf("nginx: reload failed: %w", err), rollbackErr)
	}
	return nil
}

type snapshot struct {
	content      []byte
	configExists bool
	enabled      bool
}

func (m *Manager) snapshot(configPath, enabledPath string) (snapshot, error) {
	var state snapshot
	content, err := m.files.Read(configPath)
	switch {
	case err == nil:
		state.content = content
		state.configExists = true
	case errors.Is(err, os.ErrNotExist):
	default:
		return snapshot{}, err
	}
	enabled, err := m.links.Exists(enabledPath)
	if err != nil {
		return snapshot{}, err
	}
	state.enabled = enabled
	return state, nil
}

func (m *Manager) mutate(action Action, site Site, configPath, enabledPath string) error {
	switch action {
	case ActionUpsert:
		if err := m.files.Write(configPath, render(site), 0o640); err != nil {
			return err
		}
		return m.setEnabled(configPath, enabledPath, true)
	case ActionEnable:
		return m.setEnabled(configPath, enabledPath, true)
	case ActionDisable:
		return m.setEnabled(configPath, enabledPath, false)
	case ActionDelete:
		if err := m.setEnabled(configPath, enabledPath, false); err != nil {
			return err
		}
		err := m.files.Delete(configPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}

func (m *Manager) rollback(ctx context.Context, configPath, enabledPath string, before snapshot) error {
	if err := m.restore(configPath, enabledPath, before); err != nil {
		return fmt.Errorf("nginx: rollback files: %w", err)
	}
	if _, err := m.runner.Run(ctx, "nginx", "-t"); err != nil {
		return fmt.Errorf("nginx: restored configuration is invalid: %w", err)
	}
	if _, err := m.runner.Run(ctx, "nginx", "-s", "reload"); err != nil {
		return fmt.Errorf("nginx: reload restored configuration: %w", err)
	}
	return nil
}

func (m *Manager) restore(configPath, enabledPath string, before snapshot) error {
	var errs []error
	if before.configExists {
		errs = append(errs, m.files.Write(configPath, before.content, 0o640))
	} else {
		err := m.files.Delete(configPath)
		if !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	errs = append(errs, m.setEnabled(configPath, enabledPath, before.enabled))
	return errors.Join(errs...)
}

func (m *Manager) setEnabled(configPath, enabledPath string, enabled bool) error {
	exists, err := m.links.Exists(enabledPath)
	if err != nil {
		return err
	}
	if enabled {
		if exists {
			return nil
		}
		return m.links.Symlink(configPath, enabledPath)
	}
	if !exists {
		return nil
	}
	return m.links.Remove(enabledPath)
}

func (m *Manager) paths(domain string) (string, string) {
	filename := "opendeploy-" + strings.ToLower(domain) + ".conf"
	return path.Join(m.availableDir, filename), path.Join(m.enabledDir, filename)
}

var (
	domainPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$`)
	phpPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	pathPattern   = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
)

func validate(action Action, site Site) error {
	if action != ActionUpsert && action != ActionDelete && action != ActionEnable && action != ActionDisable {
		return fmt.Errorf("nginx: unsupported site action %q", action)
	}
	if len(site.Domain) == 0 || len(site.Domain) > 253 || !domainPattern.MatchString(site.Domain) ||
		strings.Contains(site.Domain, "..") || strings.Contains(site.Domain, ".-") || strings.Contains(site.Domain, "-.") {
		return fmt.Errorf("nginx: invalid domain")
	}
	if action != ActionUpsert {
		return nil
	}
	cleanRoot := path.Clean(site.RootPath)
	if cleanRoot != site.RootPath || !pathPattern.MatchString(cleanRoot) ||
		(!strings.HasPrefix(cleanRoot, "/var/www/") && !strings.HasPrefix(cleanRoot, "/srv/")) {
		return fmt.Errorf("nginx: root path is outside managed roots")
	}
	if site.PHPVersion != "" && !phpPattern.MatchString(site.PHPVersion) {
		return fmt.Errorf("nginx: invalid PHP version")
	}
	if site.SSLEnabled && (site.SSLCert == "" || site.SSLKey == "") {
		return fmt.Errorf("nginx: SSL certificate and key are required")
	}
	if site.SSLEnabled {
		for _, certificatePath := range []string{site.SSLCert, site.SSLKey} {
			clean := path.Clean(certificatePath)
			if clean != certificatePath || !pathPattern.MatchString(clean) ||
				(!strings.HasPrefix(clean, "/etc/letsencrypt/") &&
					!strings.HasPrefix(clean, "/var/lib/opendeploy/")) {
				return fmt.Errorf("nginx: certificate path is outside managed roots")
			}
		}
	}
	return nil
}

func render(site Site) []byte {
	var b strings.Builder
	b.WriteString("# Managed by OpenDeploy. Manual changes will be overwritten.\n")
	b.WriteString("server {\n")
	if site.SSLEnabled {
		b.WriteString("    listen 443 ssl;\n")
		b.WriteString("    listen [::]:443 ssl;\n")
	} else {
		b.WriteString("    listen 80;\n")
		b.WriteString("    listen [::]:80;\n")
	}
	fmt.Fprintf(&b, "    server_name %s;\n", strings.ToLower(site.Domain))
	fmt.Fprintf(&b, "    root %s;\n", site.RootPath)
	b.WriteString("    index index.html index.htm index.php;\n\n")
	if site.SSLEnabled {
		fmt.Fprintf(&b, "    ssl_certificate %s;\n", site.SSLCert)
		fmt.Fprintf(&b, "    ssl_certificate_key %s;\n\n", site.SSLKey)
	}
	b.WriteString("    location / {\n        try_files $uri $uri/ /index.php?$query_string;\n    }\n")
	if site.PHPVersion != "" {
		b.WriteString("\n    location ~ \\.php$ {\n")
		b.WriteString("        include snippets/fastcgi-php.conf;\n")
		fmt.Fprintf(&b, "        fastcgi_pass unix:/run/php/php%s-fpm.sock;\n", site.PHPVersion)
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	return []byte(b.String())
}
