package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/anrted/opendeploy/pkg/contract"
)

const moduleID = "firewall"

type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Firewall" }
func (m *Module) Version() string     { return "2.0.0" }
func (m *Module) Description() string { return "Advanced Firewall Management" }

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	
	// Ensure critical ports are allowed so we don't lock ourselves out if UFW is enabled
	criticalPorts := []struct {
		port  string
		proto string
	}{
		{"22", "tcp"},
		{"80", "tcp"},
		{"443", "tcp"},
		{"5888", "tcp"},
	}

	for _, p := range criticalPorts {
		req := &contract.FirewallRuleRequest{
			Port:     p.port,
			Protocol: p.proto,
			Action:   "allow",
		}
		if err := m.deps.Agent.FirewallRule(context.Background(), req); err != nil {
			m.logger.Warn("failed to ensure critical port is allowed", "port", p.port, "proto", p.proto, "error", err)
		}
	}
	
	m.logger.Info("firewall module bootstrapped")
	return nil
}

func (m *Module) Shutdown(_ context.Context) error { return nil }

func (m *Module) RegisterRoutes(r contract.Router) {
	r.Get("/status", m.handleStatus)
	r.Get("/rules", m.handleList)
	r.Post("/rules", m.handleAdd)
	r.Delete("/rules", m.handleDelete)
	r.Post("/toggle", m.handleToggle)
	r.Post("/reset", m.handleReset)
}

func (m *Module) RegisterMenuItems() []contract.MenuItem {
	return []contract.MenuItem{
		{ID: moduleID, Label: "Firewall", Icon: "shield", Path: "/modules/firewall", Order: 40},
	}
}

func (m *Module) RegisterSettings() []contract.SettingSpec {
	return nil
}

func (m *Module) Install(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: install")
	return nil
}

func (m *Module) Uninstall(ctx context.Context) error {
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: enable")
	return m.deps.Agent.FirewallToggle(ctx, true)
}

func (m *Module) Disable(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: disable")
	return m.deps.Agent.FirewallToggle(ctx, false)
}

func (m *Module) Restart(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: restart")
	return nil
}

func (m *Module) Status(ctx context.Context) (*contract.ModuleStatus, error) {
	st, err := m.deps.Agent.FirewallStatus(ctx)
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
	st, err := m.deps.Agent.FirewallStatus(ctx)
	if err != nil || !st.Active {
		return &contract.HealthReport{Status: contract.HealthWarning, Message: "Firewall is not running"}, nil
	}
	return &contract.HealthReport{Status: contract.HealthOK}, nil
}

// Handlers

func (m *Module) handleStatus(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	status, err := m.deps.Agent.FirewallStatus(r.Context())
	if err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]any{"status": status})
}

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

func (m *Module) handleAdd(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	var req contract.FirewallRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, fmt.Errorf("invalid json body"))
		return
	}

	if req.Action == "" {
		req.Action = "allow"
	}

	if err := m.deps.Agent.FirewallRule(r.Context(), &req); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule added successfully"})
}

type deleteRequest struct {
	ID string `json:"id"`
}

func (m *Module) handleDelete(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	
	// Support both path params (not easily available in this router) or JSON body
	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, fmt.Errorf("invalid json body"))
		return
	}

	if req.ID == "" {
		m.writeError(w, fmt.Errorf("rule id is required"))
		return
	}

	if err := m.deps.Agent.FirewallDelete(r.Context(), req.ID); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule deleted successfully"})
}

type toggleRequest struct {
	Enable bool `json:"enable"`
}

func (m *Module) handleToggle(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	var req toggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, fmt.Errorf("invalid json body"))
		return
	}

	if err := m.deps.Agent.FirewallToggle(r.Context(), req.Enable); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "firewall toggled successfully"})
}

func (m *Module) handleReset(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)

	if err := m.deps.Agent.FirewallReset(r.Context()); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "firewall reset successfully"})
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
