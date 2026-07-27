package site

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/pkg/contract"
)

type mockWebServer struct {
	actions []contract.SiteAction
	fail    error
}

func (m *mockWebServer) ID() string                               { return "nginx" }
func (m *mockWebServer) Name() string                             { return "Mock Nginx" }
func (m *mockWebServer) Version() string                          { return "1.0" }
func (m *mockWebServer) Description() string                      { return "Mock" }
func (m *mockWebServer) Bootstrap(deps contract.ModuleDeps) error { return nil }
func (m *mockWebServer) Shutdown(ctx context.Context) error       { return nil }
func (m *mockWebServer) RegisterRoutes(r contract.Router)         {}
func (m *mockWebServer) RegisterMenuItems() []contract.MenuItem   { return nil }
func (m *mockWebServer) RegisterSettings() []contract.SettingSpec { return nil }
func (m *mockWebServer) Install(ctx context.Context) error        { return nil }
func (m *mockWebServer) Uninstall(ctx context.Context) error      { return nil }
func (m *mockWebServer) Enable(ctx context.Context) error         { return nil }
func (m *mockWebServer) Disable(ctx context.Context) error        { return nil }
func (m *mockWebServer) Restart(ctx context.Context) error        { return nil }
func (m *mockWebServer) Category() string                         { return "Web" }
func (m *mockWebServer) Icon() string                             { return "server" }
func (m *mockWebServer) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}
func (m *mockWebServer) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{}
}
func (m *mockWebServer) Actions() []contract.ActionDef                               { return nil }
func (m *mockWebServer) ExecuteAction(ctx context.Context, actionID string) error    { return nil }
func (m *mockWebServer) Logs() []contract.LogDef                                     { return nil }
func (m *mockWebServer) SettingsSchema() []contract.SettingField                     { return nil }
func (m *mockWebServer) Pages() []contract.ModulePage                                { return nil }
func (m *mockWebServer) Status(ctx context.Context) (*contract.RuntimeStatus, error) { return nil, nil }
func (m *mockWebServer) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	return nil, nil
}

func (m *mockWebServer) ApplySite(_ context.Context, action contract.SiteAction, _ contract.SiteSpec) error {
	m.actions = append(m.actions, action)
	err := m.fail
	m.fail = nil
	return err
}

type lifecycleRepo struct {
	Repository
	created *Site
	fail    error
}

func (r *lifecycleRepo) Create(_ context.Context, site *Site) error {
	if r.fail != nil {
		return r.fail
	}
	site.ID = "site-1"
	r.created = site
	return nil
}

type mockAgent struct {
	contract.AgentClient
}

func (m *mockAgent) DirCreate(ctx context.Context, path string, mode uint32) error  { return nil }
func (m *mockAgent) FileChown(ctx context.Context, path string, uid, gid int) error { return nil }

func TestCreateCompensatesNginxWhenPersistenceFails(t *testing.T) {
	repo := &lifecycleRepo{fail: errors.New("database unavailable")}
	agent := &mockAgent{}
	mockWeb := &mockWebServer{}
	registry := module.NewRegistry()
	registry.Register(mockWeb)
	service := NewService(repo, nil, agent, registry, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Create(context.Background(), CreateRequest{
		Domain: "example.com", RootPath: "/var/www/example", ModuleID: "nginx",
	}, "user-1", "127.0.0.1")
	if err == nil {
		t.Fatal("Create succeeded, want repository error")
	}
	if got, want := mockWeb.actions, []contract.SiteAction{contract.SiteUpsert, contract.SiteDelete}; !equalActions(got, want) {
		t.Fatalf("agent actions = %v, want %v", got, want)
	}
}

func TestCreateDoesNotPersistWhenNginxValidationFails(t *testing.T) {
	repo := &lifecycleRepo{}
	agent := &mockAgent{}
	mockWeb := &mockWebServer{fail: errors.New("nginx -t failed")}
	registry := module.NewRegistry()
	registry.Register(mockWeb)
	service := NewService(repo, nil, agent, registry, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Create(context.Background(), CreateRequest{
		Domain: "example.com", RootPath: "/var/www/example", ModuleID: "nginx",
	}, "user-1", "127.0.0.1")
	if err == nil {
		t.Fatal("Create succeeded, want nginx error")
	}
	if repo.created != nil {
		t.Fatal("site was persisted after nginx validation failure")
	}
	if got, want := mockWeb.actions, []contract.SiteAction{contract.SiteUpsert}; !equalActions(got, want) {
		t.Fatalf("agent actions = %v, want %v", got, want)
	}
}

func equalActions(left, right []contract.SiteAction) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
