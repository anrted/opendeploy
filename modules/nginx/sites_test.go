package nginx

import (
	"strings"
	"testing"

	"github.com/anrted/opendeploy/pkg/contract"
)

func TestRenderNginx_ReverseProxy(t *testing.T) {
	spec := contract.SiteSpec{
		PrimaryDomain: "panel.xeber.ru",
		RootPath:      "/var/www/panel.xeber.ru",
		ProxyEnabled:  true,
		ProxyHost:     "127.0.0.1",
		ProxyPort:     8080,
	}

	content, err := renderNginx(spec)
	if err != nil {
		t.Fatalf("Failed to render nginx template: %v", err)
	}

	result := string(content)

	if !strings.Contains(result, "server_name panel.xeber.ru;") {
		t.Errorf("Missing server_name directive, got:\n%s", result)
	}

	if !strings.Contains(result, "proxy_pass http://127.0.0.1:8080;") {
		t.Errorf("Missing proxy_pass directive, got:\n%s", result)
	}

	if !strings.Contains(result, "proxy_set_header X-Forwarded-For") {
		t.Errorf("Missing proxy headers, got:\n%s", result)
	}

	if strings.Contains(result, "try_files $uri") {
		t.Errorf("Should not contain try_files for reverse proxy, got:\n%s", result)
	}
}

func TestRenderNginx_Static(t *testing.T) {
	spec := contract.SiteSpec{
		PrimaryDomain: "static.xeber.ru",
		RootPath:      "/var/www/static.xeber.ru",
		AppType:       "static",
	}

	content, err := renderNginx(spec)
	if err != nil {
		t.Fatalf("Failed to render nginx template: %v", err)
	}

	result := string(content)
	if !strings.Contains(result, "try_files $uri $uri/ =404;") {
		t.Errorf("Missing try_files directive for static site, got:\n%s", result)
	}
	if strings.Contains(result, "proxy_pass") {
		t.Errorf("Should not contain proxy_pass for static site, got:\n%s", result)
	}
}
