package nginx

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/anrted/opendeploy/pkg/contract"
)

//go:embed templates/site.conf.tmpl
var siteConfigTemplate string

var parsedSiteTemplate = template.Must(template.New("site").Parse(siteConfigTemplate))

func (m *Module) ApplySite(ctx context.Context, action contract.SiteAction, site contract.SiteSpec) error {
	if !validManagedDomain(site.PrimaryDomain) {
		return fmt.Errorf("nginx: invalid primary domain")
	}
	configPath := fmt.Sprintf("/etc/nginx/sites-available/opendeploy-%s.conf", site.PrimaryDomain)
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/opendeploy-%s.conf", site.PrimaryDomain)
	switch action {
	case contract.SiteUpsert, contract.SiteEnable:
		configSnapshot := m.readSnapshot(ctx, configPath)
		enabledSnapshot := m.readSnapshot(ctx, enabledPath)
		if action == contract.SiteUpsert {
			content, err := renderNginx(site)
			if err != nil {
				return err
			}
			if err := m.deps.Agent.FileWrite(ctx, configPath, content, 0o644); err != nil {
				return fmt.Errorf("nginx: write config: %w", err)
			}
			if err := m.deps.Agent.FileWrite(ctx, enabledPath, content, 0o644); err != nil {
				m.restoreSnapshot(ctx, configPath, configSnapshot)
				return fmt.Errorf("nginx: enable site: %w", err)
			}
		} else if err := m.deps.Agent.FileCopy(ctx, configPath, enabledPath); err != nil {
			return fmt.Errorf("nginx: enable site: %w", err)
		}
		exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil || exitCode != 0 {
			m.restoreSnapshot(ctx, configPath, configSnapshot)
			m.restoreSnapshot(ctx, enabledPath, enabledSnapshot)
			return fmt.Errorf("nginx config test failed: %s\n%s", stdout, stderr)
		}
		if err := m.deps.Agent.ServiceReload(ctx, "nginx"); err != nil {
			m.restoreSnapshot(ctx, configPath, configSnapshot)
			m.restoreSnapshot(ctx, enabledPath, enabledSnapshot)
			return fmt.Errorf("nginx: reload after site change: %w", err)
		}
		return nil
	case contract.SiteDisable:
		enabledSnapshot := m.readSnapshot(ctx, enabledPath)
		if err := m.deps.Agent.FileDelete(ctx, enabledPath); err != nil {
			return fmt.Errorf("nginx: disable site: %w", err)
		}
		if err := m.validateAndReload(ctx); err != nil {
			m.restoreSnapshot(ctx, enabledPath, enabledSnapshot)
			return err
		}
		return nil
	case contract.SiteDelete:
		configSnapshot := m.readSnapshot(ctx, configPath)
		enabledSnapshot := m.readSnapshot(ctx, enabledPath)
		if err := m.deps.Agent.FileDelete(ctx, enabledPath); err != nil {
			return fmt.Errorf("nginx: delete enabled site: %w", err)
		}
		if err := m.deps.Agent.FileDelete(ctx, configPath); err != nil {
			m.restoreSnapshot(ctx, enabledPath, enabledSnapshot)
			return fmt.Errorf("nginx: delete site config: %w", err)
		}
		if err := m.validateAndReload(ctx); err != nil {
			m.restoreSnapshot(ctx, configPath, configSnapshot)
			m.restoreSnapshot(ctx, enabledPath, enabledSnapshot)
			return err
		}
		return nil
	}
	return nil
}

type fileSnapshot struct {
	content []byte
	exists  bool
}

func (m *Module) readSnapshot(ctx context.Context, path string) fileSnapshot {
	content, err := m.deps.Agent.FileRead(ctx, path)
	return fileSnapshot{content: content, exists: err == nil}
}

func (m *Module) restoreSnapshot(ctx context.Context, path string, snapshot fileSnapshot) {
	if snapshot.exists {
		_ = m.deps.Agent.FileWrite(ctx, path, snapshot.content, 0o644)
		return
	}
	_ = m.deps.Agent.FileDelete(ctx, path)
}

func (m *Module) validateAndReload(ctx context.Context) error {
	exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("nginx config test failed: %s\n%s", stdout, stderr)
	}
	if err := m.deps.Agent.ServiceReload(ctx, "nginx"); err != nil {
		return fmt.Errorf("nginx: reload: %w", err)
	}
	return nil
}

func renderNginx(site contract.SiteSpec) ([]byte, error) {
	var output bytes.Buffer
	if err := parsedSiteTemplate.Execute(&output, site); err != nil {
		return nil, fmt.Errorf("failed to render nginx template: %w", err)
	}
	return output.Bytes(), nil
}
