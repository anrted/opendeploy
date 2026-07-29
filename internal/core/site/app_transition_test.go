package site

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/pkg/contract"
)

type transitionRepo struct {
	Repository
	site    Site
	updated *Site
}

func (r *transitionRepo) FindByID(context.Context, string) (*Site, error) {
	current := r.site
	return &current, nil
}

func (r *transitionRepo) ListAll(context.Context) ([]Site, error) {
	return []Site{r.site}, nil
}

func (r *transitionRepo) Update(_ context.Context, current *Site) error {
	copy := *current
	r.updated = &copy
	return nil
}

type mockAppServer struct {
	contract.Module
	actions []contract.SiteAction
}

func (m *mockAppServer) ID() string { return "php" }

func (m *mockAppServer) ApplyApp(_ context.Context, action contract.SiteAction, _ contract.SiteSpec) error {
	m.actions = append(m.actions, action)
	return nil
}

func newTransitionService(repo Repository, web *mockWebServer, app *mockAppServer) *Service {
	registry := module.NewRegistry()
	registry.Register(web)
	registry.Register(app)
	return NewService(repo, nil, &mockAgent{}, registry, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func phpSite() Site {
	version := "8.3"
	return Site{
		ID: "site-1", Name: "example.com", ModuleID: "nginx", RootPath: "/var/www/example",
		State:   StateActive,
		Domains: []Domain{{Domain: "example.com", Type: DomainPrimary}},
		App:     App{AppType: "php", AppVersion: &version},
	}
}

func TestUpdateFromPHPToStaticRemovesPoolAndClearsVersion(t *testing.T) {
	repo := &transitionRepo{site: phpSite()}
	web := &mockWebServer{}
	app := &mockAppServer{}
	service := newTransitionService(repo, web, app)
	static := "static"

	if _, err := service.Update(context.Background(), "site-1", UpdateRequest{AppType: &static}, "user-1", "127.0.0.1"); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got, want := app.actions, []contract.SiteAction{contract.SiteDelete}; !equalActions(got, want) {
		t.Fatalf("app actions = %v, want %v", got, want)
	}
	if repo.updated == nil || repo.updated.App.AppType != "static" || repo.updated.App.AppVersion != nil {
		t.Fatalf("updated app = %#v, want static without a version", repo.updated)
	}
}

func TestReconcileActiveRepairsPHPAndNginxConfiguration(t *testing.T) {
	repo := &transitionRepo{site: phpSite()}
	web := &mockWebServer{}
	app := &mockAppServer{}
	service := newTransitionService(repo, web, app)

	if err := service.ReconcileActive(context.Background()); err != nil {
		t.Fatalf("ReconcileActive returned error: %v", err)
	}
	if got, want := app.actions, []contract.SiteAction{contract.SiteUpsert}; !equalActions(got, want) {
		t.Fatalf("app actions = %v, want %v", got, want)
	}
	if got, want := web.actions, []contract.SiteAction{contract.SiteUpsert}; !equalActions(got, want) {
		t.Fatalf("web actions = %v, want %v", got, want)
	}
}
