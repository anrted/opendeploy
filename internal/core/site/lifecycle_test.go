package site

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/anrted/opendeploy/pkg/contract"
)

type lifecycleAgent struct {
	contract.AgentClient
	actions []contract.NginxSiteAction
	fail    error
}

func (a *lifecycleAgent) NginxSiteApply(_ context.Context, action contract.NginxSiteAction, _ contract.NginxSiteSpec) error {
	a.actions = append(a.actions, action)
	err := a.fail
	a.fail = nil
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

func TestCreateCompensatesNginxWhenPersistenceFails(t *testing.T) {
	repo := &lifecycleRepo{fail: errors.New("database unavailable")}
	agent := &lifecycleAgent{}
	service := NewService(repo, nil, agent, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Create(context.Background(), CreateRequest{
		Domain: "example.com", RootPath: "/var/www/example", ModuleID: "nginx",
	}, "user-1", "127.0.0.1")
	if err == nil {
		t.Fatal("Create succeeded, want repository error")
	}
	if got, want := agent.actions, []contract.NginxSiteAction{contract.NginxSiteUpsert, contract.NginxSiteDelete}; !equalActions(got, want) {
		t.Fatalf("agent actions = %v, want %v", got, want)
	}
}

func TestCreateDoesNotPersistWhenNginxValidationFails(t *testing.T) {
	repo := &lifecycleRepo{}
	agent := &lifecycleAgent{fail: errors.New("nginx -t failed")}
	service := NewService(repo, nil, agent, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Create(context.Background(), CreateRequest{
		Domain: "example.com", RootPath: "/var/www/example", ModuleID: "nginx",
	}, "user-1", "127.0.0.1")
	if err == nil {
		t.Fatal("Create succeeded, want nginx error")
	}
	if repo.created != nil {
		t.Fatal("site was persisted after nginx validation failure")
	}
	if got, want := agent.actions, []contract.NginxSiteAction{contract.NginxSiteUpsert}; !equalActions(got, want) {
		t.Fatalf("agent actions = %v, want %v", got, want)
	}
}

func equalActions(left, right []contract.NginxSiteAction) bool {
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
