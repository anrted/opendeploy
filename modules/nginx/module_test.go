package nginx

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/pkg/contract"
)

type nginxTestAgent struct {
	contract.AgentClient
	files      map[string][]byte
	commandErr error
	exitCode   int
	reloads    int
}

type typedNginxTestAgent struct {
	*nginxTestAgent
	action  contract.SiteAction
	domain  string
	content []byte
}

func (a *typedNginxTestAgent) NginxSiteApply(_ context.Context, action contract.SiteAction, domain string, content []byte) error {
	a.action = action
	a.domain = domain
	a.content = append([]byte(nil), content...)
	return nil
}

func (a *nginxTestAgent) FileRead(_ context.Context, path string) ([]byte, error) {
	content, ok := a.files[path]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), content...), nil
}

func (a *nginxTestAgent) FileWrite(_ context.Context, path string, content []byte, _ uint32) error {
	a.files[path] = append([]byte(nil), content...)
	return nil
}

func (a *nginxTestAgent) FileDelete(_ context.Context, path string) error {
	delete(a.files, path)
	return nil
}

func (a *nginxTestAgent) FileCopy(_ context.Context, source, destination string) error {
	content, ok := a.files[source]
	if !ok {
		return errors.New("source not found")
	}
	a.files[destination] = append([]byte(nil), content...)
	return nil
}

func (a *nginxTestAgent) CommandExecute(_ context.Context, _ string, _ ...string) (int, string, string, error) {
	return a.exitCode, "", "invalid config", a.commandErr
}

func (a *nginxTestAgent) ServiceReload(_ context.Context, _ string) error {
	a.reloads++
	return nil
}

func (a *nginxTestAgent) PackageInstalled(_ context.Context, _ string) (bool, string, error) {
	return true, "1.26.0", nil
}

func testNginxModule(agent contract.AgentClient) *Module {
	module := New()
	_ = module.Bootstrap(contract.ModuleDeps{
		Agent:  agent,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return module
}

func TestNginxImplementsDynamicProviders(t *testing.T) {
	var module any = New()
	if _, ok := module.(contract.DataGridProvider); !ok {
		t.Fatal("nginx does not implement DataGridProvider")
	}
	if _, ok := module.(contract.SettingsProvider); !ok {
		t.Fatal("nginx does not implement SettingsProvider")
	}
	if _, ok := module.(contract.LogProvider); !ok {
		t.Fatal("nginx does not implement LogProvider")
	}
}

func TestSiteActionsAreRowActions(t *testing.T) {
	schema, err := New().DataGridSchema("sites")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Actions) != 0 || len(schema.RowActions) != 3 {
		t.Fatalf("site actions: global=%d row=%d, want 0 and 3", len(schema.Actions), len(schema.RowActions))
	}
}

func TestApplySiteRestoresPreviousConfigurationWhenValidationFails(t *testing.T) {
	const (
		available = "/etc/nginx/sites-available/opendeploy-example.com.conf"
		enabled   = "/etc/nginx/sites-enabled/opendeploy-example.com.conf"
	)
	agent := &nginxTestAgent{
		files:    map[string][]byte{available: []byte("old available"), enabled: []byte("old enabled")},
		exitCode: 1,
	}
	module := testNginxModule(agent)
	err := module.ApplySite(context.Background(), contract.SiteUpsert, contract.SiteSpec{
		Name: "example", PrimaryDomain: "example.com", RootPath: "/var/www/example", AppType: "static",
	})
	if err == nil {
		t.Fatal("ApplySite succeeded with an invalid configuration")
	}
	if got := string(agent.files[available]); got != "old available" {
		t.Fatalf("available config = %q, want restored snapshot", got)
	}
	if got := string(agent.files[enabled]); got != "old enabled" {
		t.Fatalf("enabled config = %q, want restored snapshot", got)
	}
}

func TestEnableSiteCopiesAvailableConfiguration(t *testing.T) {
	const (
		available = "/etc/nginx/sites-available/opendeploy-example.com.conf"
		enabled   = "/etc/nginx/sites-enabled/opendeploy-example.com.conf"
	)
	agent := &nginxTestAgent{files: map[string][]byte{available: []byte("server {}")}}
	module := testNginxModule(agent)
	if err := module.ApplySite(context.Background(), contract.SiteEnable, contract.SiteSpec{PrimaryDomain: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(agent.files[available], agent.files[enabled]) {
		t.Fatal("enabled configuration is not a copy of sites-available")
	}
	if agent.reloads != 1 {
		t.Fatalf("reload count = %d, want 1", agent.reloads)
	}
}

func TestApplySiteUsesTypedAgentOperation(t *testing.T) {
	agent := &typedNginxTestAgent{nginxTestAgent: &nginxTestAgent{files: make(map[string][]byte)}}
	module := testNginxModule(agent)
	ctx := servercontext.WithID(context.Background(), "remote-test")
	if err := module.ApplySite(ctx, contract.SiteUpsert, contract.SiteSpec{
		Name: "example", PrimaryDomain: "example.com", RootPath: "/var/www/example",
		AppType: "proxy", ProxyTarget: "http://127.0.0.1:3000", ForceHTTPS: true,
	}); err != nil {
		t.Fatal(err)
	}
	if agent.action != contract.SiteUpsert || agent.domain != "example.com" {
		t.Fatalf("typed operation = %q %q", agent.action, agent.domain)
	}
	config := string(agent.content)
	if !strings.Contains(config, "proxy_pass http://127.0.0.1:3000") {
		t.Fatalf("typed operation did not receive rendered proxy config:\n%s", config)
	}
	if len(agent.files) != 0 || agent.reloads != 0 {
		t.Fatal("module used primitive file or service operations despite typed Agent support")
	}
}

func TestPHPIndexTakesPriorityOverStaticIndex(t *testing.T) {
	const available = "/etc/nginx/sites-available/opendeploy-example.com.conf"
	agent := &nginxTestAgent{files: make(map[string][]byte)}
	module := testNginxModule(agent)

	err := module.ApplySite(context.Background(), contract.SiteUpsert, contract.SiteSpec{
		Name: "example", PrimaryDomain: "example.com", RootPath: "/var/www/example",
		AppType: "php", AppVersion: "8.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	config := string(agent.files[available])
	if !strings.Contains(config, "index index.php index.html index.htm;") {
		t.Fatalf("PHP index does not have priority:\n%s", config)
	}
}

func TestValidateNginxSettings(t *testing.T) {
	valid := validNginxSettings()
	if _, err := validateNginxSettings(valid); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	invalid := make(map[string]any, len(valid))
	for key, value := range valid {
		invalid[key] = value
	}
	invalid["access_log"] = "/tmp/nginx.log"
	if _, err := validateNginxSettings(invalid); err == nil {
		t.Fatal("unsafe access log path was accepted")
	}
}

func validNginxSettings() map[string]any {
	return map[string]any{
		"worker_processes": "auto", "worker_connections": "2048", "keepalive_timeout": "65",
		"client_max_body_size": "50m", "sendfile": true, "gzip": true,
		"gzip_types": "text/plain application/json", "server_tokens": false,
		"access_log": "/var/log/nginx/access.log", "error_log": "/var/log/nginx/error.log warn",
	}
}

func TestSaveSettingsRestoresBothFilesWhenValidationFails(t *testing.T) {
	const oldMain = `worker_processes auto;
events {
    worker_connections 1024;
}
http {
    include /etc/nginx/conf.d/*.conf;
}`
	const oldManaged = "keepalive_timeout 30;\n"
	agent := &nginxTestAgent{
		files: map[string][]byte{
			nginxMainConfigPath:    []byte(oldMain),
			nginxManagedConfigPath: []byte(oldManaged),
		},
		exitCode: 1,
	}
	module := testNginxModule(agent)
	if err := module.SaveSettings(context.Background(), validNginxSettings()); err == nil {
		t.Fatal("SaveSettings succeeded with failed nginx validation")
	}
	if got := string(agent.files[nginxMainConfigPath]); got != oldMain {
		t.Fatalf("nginx.conf was not restored:\n%s", got)
	}
	if got := string(agent.files[nginxManagedConfigPath]); got != oldManaged {
		t.Fatalf("managed settings were not restored:\n%s", got)
	}
}

func TestSaveConfigurationFileRollsBackInvalidEdit(t *testing.T) {
	const filePath = "/etc/nginx/conf.d/security.conf"
	agent := &nginxTestAgent{files: map[string][]byte{filePath: []byte("server_tokens off;\n")}, exitCode: 1}
	module := testNginxModule(agent)
	if err := module.saveConfigurationFile(context.Background(), filePath, "broken directive;\n"); err == nil {
		t.Fatal("invalid configuration edit succeeded")
	}
	if got := string(agent.files[filePath]); got != "server_tokens off;\n" {
		t.Fatalf("configuration was not restored: %q", got)
	}
}

func TestReplaceNginxMainDirectivesPreservesOtherConfiguration(t *testing.T) {
	input := `user www-data;
worker_processes auto;
events {
    worker_connections 768;
}
http {
    include /etc/nginx/conf.d/*.conf;
}`
	updated, err := replaceDirectiveAtDepth(input, "worker_processes", "4", 0)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = replaceDirectiveInBlock(updated, "events", "worker_connections", "4096")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"user www-data;", "worker_processes 4;", "worker_connections 4096;", "include /etc/nginx/conf.d/*.conf;"} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated config does not contain %q:\n%s", expected, updated)
		}
	}
}

func TestConfigurationPathValidation(t *testing.T) {
	for _, valid := range []string{
		"/etc/nginx/nginx.conf",
		"/etc/nginx/mime.types",
		"/etc/nginx/conf.d/security.conf",
		"/etc/nginx/sites-available/example.conf",
		"/etc/nginx/snippets/fastcgi.conf",
	} {
		if !isEditableNginxConfig(valid) {
			t.Errorf("valid configuration path rejected: %s", valid)
		}
	}
	for _, invalid := range []string{
		"/etc/passwd",
		"/etc/nginx/sites-enabled/example.conf",
		"/etc/nginx/conf.d/../nginx.conf",
		"/etc/nginx/conf.d/nested/example.conf",
	} {
		if isEditableNginxConfig(invalid) {
			t.Errorf("unsafe configuration path accepted: %s", invalid)
		}
	}
}

func TestCertificateMetadata(t *testing.T) {
	row := map[string]any{}
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	applyCertificateMetadata(row, `issuer=CN = Test CA
notBefore=Jul 1 00:00:00 2026 GMT
notAfter=Aug 20 00:00:00 2026 GMT
X509v3 Subject Alternative Name:
    DNS:example.com, DNS:www.example.com`, now)
	if row["status"] != "expiring" {
		t.Fatalf("certificate status = %v, want expiring", row["status"])
	}
	if !strings.Contains(row["san"].(string), "www.example.com") {
		t.Fatalf("unexpected SAN: %v", row["san"])
	}
}
