package ufw

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "ufw"

type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "UFW Firewall" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Uncomplicated Firewall management" }

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	
	// Ensure critical ports are allowed so we don't lock ourselves out if UFW is enabled
	criticalPorts := []struct {
		port  int
		proto string
	}{
		{22, "tcp"},
		{80, "tcp"},
		{443, "tcp"},
		{5888, "tcp"},
	}

	for _, p := range criticalPorts {
		if err := m.deps.Agent.FirewallAllow(context.Background(), p.port, p.proto); err != nil {
			m.logger.Warn("failed to ensure critical port is allowed", "port", p.port, "proto", p.proto, "error", err)
		}
	}
	
	m.logger.Info("ufw module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }

func (m *Module) RegisterRoutes(r contract.Router) {
	r.Get("/rules", m.handleList)
	r.Post("/rules", m.handleAdd)
	r.Delete("/rules", m.handleDelete)
}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "Firewall", Icon: "shield", Path: "/modules/ufw", Order: 40},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return nil
}

func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "ufw: install")
	// UFW is already installed on Ubuntu, or you could do: deps.Agent.PackageInstall
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	m.logger.InfoContext(ctx, "ufw: enable")
	return m.deps.Agent.ServiceEnable(ctx, "ufw")
}

func (m *Module) Disable(ctx context.Context) error {
	m.logger.InfoContext(ctx, "ufw: disable")
	return m.deps.Agent.ServiceDisable(ctx, "ufw")
}

func (m *Module) Restart(ctx context.Context) error {
	m.logger.InfoContext(ctx, "ufw: restart")
	return m.deps.Agent.ServiceRestart(ctx, "ufw")
}

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	st, err := m.deps.Agent.ServiceStatus(ctx, "ufw")
	if err != nil {
		return &contract.ModuleStatus{State: contract.StateError}, nil
	}
	state := contract.StateInstalled
	if st.Active {
		state = contract.StateEnabled
	}
	return &contract.ModuleStatus{State: state}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	st, err := m.deps.Agent.ServiceStatus(ctx, "ufw")
	if err != nil || !st.Active {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "UFW is not running"}, nil
	}
	return &contract.HealthReport{Status: contract.HealthOK}, nil
}

// Handlers

func (m *Module) handleList(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	rules, err := m.deps.Agent.FirewallList(r.Context())
	if err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]any{"rules": rules})
}

type ruleRequest struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"` // "allow", "deny", "delete"
}

func (m *Module) handleAdd(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	var req ruleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, fmt.Errorf("invalid json body"))
		return
	}

	if req.Port <= 0 || req.Port > 65535 {
		m.writeError(w, fmt.Errorf("invalid port"))
		return
	}
	if req.Protocol != "tcp" && req.Protocol != "udp" {
		m.writeError(w, fmt.Errorf("invalid protocol"))
		return
	}

	var err error
	if req.Action == "allow" {
		err = m.deps.Agent.FirewallAllow(r.Context(), req.Port, req.Protocol)
	} else if req.Action == "deny" {
		err = m.deps.Agent.FirewallDeny(r.Context(), req.Port, req.Protocol)
	} else {
		err = fmt.Errorf("invalid action: %s", req.Action)
	}

	if err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule added successfully"})
}

func (m *Module) handleDelete(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	var req ruleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, fmt.Errorf("invalid json body"))
		return
	}

	if err := m.deps.Agent.FirewallDelete(r.Context(), req.Port, req.Protocol); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule deleted successfully"})
}

func (m *Module) respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (m *Module) writeError(w http.ResponseWriter, err error) {
	m.logger.Error("request failed", "error", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": err.Error()}})
}
