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
		exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil || exitCode != 0 {
			_ = m.deps.Agent.FileDelete(ctx, enabledPath)
			if action == contract.SiteUpsert {
				_ = m.deps.Agent.FileDelete(ctx, configPath)
			}
			return fmt.Errorf("nginx config test failed: %s\n%s", stdout, stderr)
		}
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
	var output bytes.Buffer
	if err := parsedSiteTemplate.Execute(&output, site); err != nil {
		return nil, fmt.Errorf("failed to render nginx template: %w", err)
	}
	return output.Bytes(), nil
}
