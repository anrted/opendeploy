package firewall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/go-chi/chi/v5"
)

const moduleID = "firewall"

type Module struct {
	deps   contract.ModuleDeps
	logger *slog.Logger
}

func New() *Module { return &Module{} }

func (m *Module) ID() string          { return moduleID }
func (m *Module) Name() string        { return "Firewall" }
func (m *Module) Version() string     { return "1.0.0" }
func (m *Module) Description() string { return "Advanced Firewall Management" }

func (m *Module) Category() string { return "Security" }
func (m *Module) Icon() string     { return "shield" }
func (m *Module) Dependencies() contract.ModuleDependencies {
	return contract.ModuleDependencies{}
}

func (m *Module) Bootstrap(deps contract.ModuleDeps) error {
	m.deps = deps
	m.logger = deps.Logger.With("module", moduleID)
	m.logger.Info("firewall module bootstrapped")
	return nil
}

func (m *Module) ensureCriticalPorts(ctx context.Context) {
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
		if err := m.deps.Agent.FirewallRule(ctx, req); err != nil {
			m.logger.Warn("failed to ensure critical port is allowed", "port", p.port, "proto", p.proto, "error", err)
		}
	}
}

func (m *Module) Shutdown(_ context.Context) error { return nil }

func (m *Module) RegisterRoutes(r contract.Router) {
	r.Get("/status", m.handleStatus)
	r.Get("/rules", m.handleList)
	r.Post("/rules", m.handleAdd)
	r.Put("/rules/{id}", m.handleUpdate)
	r.Delete("/rules", m.handleDelete)
	r.Post("/toggle", m.handleToggle)
	r.Post("/reload", m.handleReload)
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
	m.ensureCriticalPorts(ctx)
	return m.deps.Agent.FirewallToggle(ctx, true)
}

func (m *Module) Disable(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: disable")
	return m.deps.Agent.FirewallToggle(ctx, false)
}

func (m *Module) Restart(ctx context.Context) error {
	m.logger.InfoContext(ctx, "firewall: restart")
	_, _, _, err := m.deps.Agent.CommandExecute(ctx, "ufw", "reload")
	return err
}

func (m *Module) handleReload(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	status, err := m.deps.Agent.FirewallStatus(r.Context())
	if err != nil {
		m.writeError(w, apperrors.Internal("failed to inspect firewall state", err))
		return
	}
	if !status.Active {
		m.writeError(w, apperrors.New(http.StatusConflict, apperrors.CodeConflict, "cannot reload an inactive firewall"))
		return
	}
	if _, _, _, err := m.deps.Agent.CommandExecute(r.Context(), "ufw", "reload"); err != nil {
		m.writeError(w, apperrors.Internal("failed to reload firewall", err))
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "firewall reloaded successfully"})
}

func (m *Module) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{
		SupportsService:  true,
		SupportsSettings: false,
		SupportsLogs:     false,
		SupportsRestart:  true,
		SupportsUpdate:   true,
	}
}

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	st, err := m.deps.Agent.FirewallStatus(ctx)

	srvStatus := contract.ServiceStopped
	if err == nil {
		if st.Active {
			srvStatus = contract.ServiceRunning
		}
	} else {
		srvStatus = contract.ServiceFailed
	}

	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "ufw")
	pkgStatus := contract.PackageNotInstalled
	if installed {
		pkgStatus = contract.PackageInstalled
	}

	return &contract.RuntimeStatus{
		PackageStatus:   pkgStatus,
		ServiceStatus:   srvStatus,
		SoftwareVersion: version,
	}, nil
}

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	st, err := m.deps.Agent.FirewallStatus(ctx)
	if err != nil {
		return &contract.HealthReport{Status: contract.HealthError, Message: "Cannot query firewall state"}, nil
	}
	if !st.Active {
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

	// Map to lowercase keys to match frontend expectations
	statusMap := map[string]any{
		"active":           status.Active,
		"default_incoming": status.DefaultIncoming,
		"default_outgoing": status.DefaultOutgoing,
		"default_routed":   status.DefaultRouted,
		"ipv6_enabled":     status.IPv6Enabled,
		"logging":          status.Logging,
		"rule_count":       status.RuleCount,
		"profile_name":     status.ProfileName,
	}

	m.respond(w, http.StatusOK, map[string]any{"status": statusMap})
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

	if err := validateRule(&req); err != nil {
		m.writeError(w, err)
		return
	}
	rules, err := m.deps.Agent.FirewallList(r.Context())
	if err != nil {
		m.writeError(w, apperrors.Internal("failed to inspect firewall rules", err))
		return
	}
	for _, rule := range rules {
		if sameRule(rule, &req) {
			m.writeError(w, apperrors.New(http.StatusConflict, apperrors.CodeConflict, "an equivalent firewall rule already exists"))
			return
		}
	}

	if err := m.deps.Agent.FirewallRule(r.Context(), &req); err != nil {
		m.writeError(w, err)
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule added successfully"})
}

func (m *Module) handleUpdate(wAny interface{}, rAny interface{}) {
	w := wAny.(http.ResponseWriter)
	r := rAny.(*http.Request)
	id := chi.URLParam(r, "id")
	if id == "" {
		m.writeError(w, apperrors.InvalidInput("rule id is required"))
		return
	}
	var req contract.FirewallRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	if err := validateRule(&req); err != nil {
		m.writeError(w, err)
		return
	}
	rules, err := m.deps.Agent.FirewallList(r.Context())
	if err != nil {
		m.writeError(w, apperrors.Internal("failed to inspect firewall rules", err))
		return
	}
	found := false
	for _, rule := range rules {
		if rule.ID == id {
			found = true
			if rule.Port == "443" || rule.Port == "5888" {
				m.writeError(w, apperrors.New(http.StatusConflict, apperrors.CodeConflict, "protected panel and HTTPS rules cannot be edited"))
				return
			}
			continue
		}
		if sameRule(rule, &req) {
			m.writeError(w, apperrors.New(http.StatusConflict, apperrors.CodeConflict, "an equivalent firewall rule already exists"))
			return
		}
	}
	if !found {
		m.writeError(w, apperrors.NotFound("firewall rule"))
		return
	}
	req.ID = id
	if err := m.deps.Agent.FirewallRule(r.Context(), &req); err != nil {
		m.writeError(w, apperrors.Internal("failed to update firewall rule", err))
		return
	}
	m.respond(w, http.StatusOK, map[string]string{"message": "rule updated successfully"})
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

	// Fetch rules to check if the user is trying to delete protected ports
	rules, err := m.deps.Agent.FirewallList(r.Context())
	if err == nil {
		for _, rule := range rules {
			if rule.ID == req.ID {
				if rule.Port == "443" || rule.Port == "5888" {
					m.writeError(w, fmt.Errorf("deleting rule for port %s is forbidden", rule.Port))
					return
				}
				break
			}
		}
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

	if req.Enable {
		m.ensureCriticalPorts(r.Context())
	}

	if err := m.deps.Agent.FirewallToggle(r.Context(), req.Enable); err != nil {
		m.writeError(w, err)
		return
	}
	status, err := m.deps.Agent.FirewallStatus(r.Context())
	if err != nil {
		m.writeError(w, apperrors.Internal("firewall changed but status verification failed", err))
		return
	}
	if status.Active != req.Enable {
		m.writeError(w, apperrors.New(http.StatusConflict, apperrors.CodeConflict, "firewall did not reach the requested state"))
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
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		err = apperrors.Internal("firewall operation failed", err)
	}
	apperrors.WriteHTTP(w, err)
}

func (m *Module) Actions() []contract.ActionDef { return nil }
func (m *Module) ExecuteAction(ctx context.Context, actionID string) error {
	return fmt.Errorf("unknown action: %s", actionID)
}
func (m *Module) Logs() []contract.LogDef {
	if m.Capabilities().SupportsService {
		return []contract.LogDef{{ID: "service", Name: "Systemd Log", Type: "systemd"}}
	}
	return nil
}
func (m *Module) SettingsSchema() []contract.SettingField { return nil }

func (m *Module) Pages() []contract.ModulePage {
	pages := []contract.ModulePage{
		{ID: "overview", Title: "Overview", Type: contract.PageTypeOverview},
	}
	if m.Capabilities().SupportsSettings {
		pages = append(pages, contract.ModulePage{ID: "settings", Title: "Settings", Type: contract.PageTypeSettings})
	}
	if m.Capabilities().SupportsLogs {
		pages = append(pages, contract.ModulePage{ID: "logs", Title: "Logs", Type: contract.PageTypeLogs})
	}
	return pages
}
