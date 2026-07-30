package nginxsite

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/anrted/opendeploy/internal/agent/executor"
	"github.com/anrted/opendeploy/internal/agent/filesystem"
	"github.com/anrted/opendeploy/internal/agent/systemd"
	"github.com/anrted/opendeploy/pkg/contract"
)

var managedDomainPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

type Manager struct {
	files   *filesystem.Manager
	shell   *executor.Shell
	systemd *systemd.Manager
}

func New(files *filesystem.Manager, shell *executor.Shell, systemdManager *systemd.Manager) *Manager {
	return &Manager{files: files, shell: shell, systemd: systemdManager}
}

type snapshot struct {
	content []byte
	exists  bool
}

func (m *Manager) Apply(ctx context.Context, action contract.SiteAction, domain string, content []byte) error {
	if !validDomain(domain) {
		return fmt.Errorf("invalid managed nginx domain")
	}
	if m == nil || m.files == nil || m.shell == nil || m.systemd == nil {
		return fmt.Errorf("nginx site manager is unavailable")
	}

	configPath := "/etc/nginx/sites-available/opendeploy-" + domain + ".conf"
	enabledPath := "/etc/nginx/sites-enabled/opendeploy-" + domain + ".conf"
	configSnapshot := m.readSnapshot(configPath)
	enabledSnapshot := m.readSnapshot(enabledPath)

	switch action {
	case contract.SiteUpsert:
		if len(content) == 0 {
			return fmt.Errorf("nginx site content is required")
		}
		if err := m.files.Write(configPath, content, 0o644); err != nil {
			return fmt.Errorf("write nginx site config: %w", err)
		}
		if err := m.files.Write(enabledPath, content, 0o644); err != nil {
			m.restore(configPath, configSnapshot)
			return fmt.Errorf("enable nginx site: %w", err)
		}
	case contract.SiteEnable:
		if !configSnapshot.exists {
			return fmt.Errorf("nginx site config does not exist")
		}
		if err := m.files.Write(enabledPath, configSnapshot.content, 0o644); err != nil {
			return fmt.Errorf("enable nginx site: %w", err)
		}
	case contract.SiteDisable:
		if err := m.files.Delete(enabledPath); err != nil {
			return fmt.Errorf("disable nginx site: %w", err)
		}
	case contract.SiteDelete:
		if err := m.files.Delete(enabledPath); err != nil {
			return fmt.Errorf("delete enabled nginx site: %w", err)
		}
		if err := m.files.Delete(configPath); err != nil {
			m.restore(enabledPath, enabledSnapshot)
			return fmt.Errorf("delete nginx site config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported nginx site action %q", action)
	}

	if err := m.validateAndReload(ctx); err != nil {
		m.restore(configPath, configSnapshot)
		m.restore(enabledPath, enabledSnapshot)
		return err
	}
	return nil
}

func validDomain(domain string) bool {
	return len(domain) <= 253 &&
		managedDomainPattern.MatchString(domain) &&
		!strings.Contains(domain, "..")
}

func (m *Manager) readSnapshot(path string) snapshot {
	content, err := m.files.Read(path)
	return snapshot{content: content, exists: err == nil}
}

func (m *Manager) restore(path string, state snapshot) {
	if state.exists {
		_ = m.files.Write(path, state.content, 0o644)
		return
	}
	_ = m.files.Delete(path)
}

func (m *Manager) validateAndReload(ctx context.Context) error {
	result, err := m.shell.Run(ctx, "nginx", "-t")
	if err != nil || result == nil || result.ExitCode != 0 {
		if result == nil {
			return fmt.Errorf("nginx configuration validation failed: %w", err)
		}
		return fmt.Errorf("nginx configuration validation failed: %s", strings.TrimSpace(result.Stderr))
	}
	if err := m.systemd.Reload(ctx, "nginx"); err != nil {
		return fmt.Errorf("reload nginx: %w", err)
	}
	return nil
}
