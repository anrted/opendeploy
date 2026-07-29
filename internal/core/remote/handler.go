package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/core/controlplane"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/version"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service          *Service
	repo             *Repository
	control          *controlplane.Manager
	controlPlanePort int
	controlPlaneCert string
}

func (h *Handler) SetControlPlane(manager *controlplane.Manager) { h.control = manager }
func (h *Handler) ConfigureControlPlane(port int, certificate string) {
	h.controlPlanePort, h.controlPlaneCert = port, certificate
}

func (h *Handler) Capabilities(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	names := []string{"dashboard", "processes", "services", "files", "firewall", "cron", "packages", "system"}
	if id != LocalServerID {
		if h.control == nil {
			names = nil
		} else {
			names = h.control.Capabilities(id)
		}
	}
	items := make([]map[string]any, 0, len(names))
	for _, name := range names {
		items = append(items, map[string]any{
			"name": name, "version": "v1", "available": true,
			"experimental": false, "deprecated": false, "required_version": "0.1.25",
		})
	}
	respond(w, http.StatusOK, map[string]any{"server_id": id, "items": items})
}

const LocalServerID = "local"

func NewHandler(service *Service, repo *Repository) *Handler {
	return &Handler{service: service, repo: repo}
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func invalid(w http.ResponseWriter, message string) {
	apperrors.WriteHTTP(w, apperrors.InvalidInput(message))
}
func internal(w http.ResponseWriter, message string, err error) {
	apperrors.WriteHTTP(w, apperrors.Internal(message, err))
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decode(r, &req); err != nil {
		invalid(w, "invalid server request")
		return
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	enrollment, err := h.service.Create(r.Context(), req, scheme+"://"+r.Host)
	if err != nil {
		invalid(w, err.Error())
		return
	}
	respond(w, http.StatusCreated, enrollment)
}

func (h *Handler) ReissueEnrollment(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	enrollment, err := h.service.ReissueEnrollment(r.Context(), chi.URLParam(r, "id"), scheme+"://"+r.Host)
	if errors.Is(err, ErrServerNotFound) {
		apperrors.WriteHTTP(w, apperrors.NotFound(err.Error()))
		return
	}
	if errors.Is(err, ErrServerNotPending) {
		invalid(w, "enrollment can only be regenerated for a pending server")
		return
	}
	if err != nil {
		internal(w, "regenerate server enrollment", err)
		return
	}
	respond(w, http.StatusCreated, enrollment)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegistrationRequest
	if err := decode(r, &req); err != nil {
		invalid(w, "invalid registration request")
		return
	}
	result, err := h.service.Register(r.Context(), req)
	if err != nil {
		apperrors.WriteHTTP(w, apperrors.New(http.StatusUnauthorized, apperrors.CodeTokenInvalid, err.Error()))
		return
	}
	if h.controlPlanePort > 0 {
		host := r.Host
		if parsed, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
			host = parsed
		}
		result.ControlPlaneAddress = net.JoinHostPort(host, fmt.Sprint(h.controlPlanePort))
		if certificate, readErr := os.ReadFile(h.controlPlaneCert); readErr == nil {
			result.ControlPlaneCA = string(certificate)
		}
	}
	respond(w, http.StatusCreated, result)
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	serverID := strings.TrimSpace(r.Header.Get("X-OpenDeploy-Agent-ID"))
	fingerprint := strings.TrimSpace(r.Header.Get("X-OpenDeploy-Cert-Fingerprint"))
	if serverID == "" || fingerprint == "" {
		apperrors.WriteHTTP(w, apperrors.Unauthorized("agent certificate identity is required"))
		return
	}
	var req HeartbeatRequest
	if err := decode(r, &req); err != nil {
		invalid(w, "invalid heartbeat request")
		return
	}
	result, err := h.service.Heartbeat(r.Context(), serverID, fingerprint, req, time.Since(start).Milliseconds())
	if err != nil {
		apperrors.WriteHTTP(w, apperrors.Unauthorized(err.Error()))
		return
	}
	respond(w, http.StatusOK, result)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 25, 100)
	offset := intQuery(r, "offset", 0, 1000000)
	page, err := h.repo.List(r.Context(), r.URL.Query().Get("query"), r.URL.Query().Get("status"), r.URL.Query().Get("tag"), r.URL.Query().Get("sort"), limit, offset)
	if err != nil {
		internal(w, "list remote servers", err)
		return
	}
	if offset == 0 && localMatches(r.URL.Query().Get("query"), r.URL.Query().Get("status"), r.URL.Query().Get("tag")) {
		page.Items = append([]Server{localServer()}, page.Items...)
		page.Total++
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
		}
	}
	respond(w, http.StatusOK, page)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "id") == LocalServerID {
		respond(w, http.StatusOK, localServer())
		return
	}
	item, err := h.repo.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		internal(w, "get remote server", err)
		return
	}
	if item == nil {
		apperrors.WriteHTTP(w, apperrors.NotFound("server not found"))
		return
	}
	respond(w, http.StatusOK, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "id") == LocalServerID {
		invalid(w, "the local server cannot be deleted")
		return
	}
	if err := h.repo.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		internal(w, "delete remote server", err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "server deleted"})
}

func localServer() Server {
	hostname, _ := os.Hostname()
	now := time.Now().UTC()
	return Server{
		ID: LocalServerID, Local: true, Name: "Localhost", Hostname: hostname, Status: "online",
		AgentVersion: version.Version, APIVersion: "v1", OS: runtime.GOOS, Architecture: runtime.GOARCH,
		HealthStatus: "healthy", UpdateChannel: "stable", LastHeartbeat: &now,
		CreatedAt: now, UpdatedAt: now,
	}
}

func localMatches(query, status, tag string) bool {
	if status != "" && status != "online" {
		return false
	}
	if tag != "" {
		return false
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	server := localServer()
	return strings.Contains(strings.ToLower(server.Name), query) ||
		strings.Contains(strings.ToLower(server.Hostname), query)
}

func (h *Handler) Action(w http.ResponseWriter, r *http.Request) {
	id, action := chi.URLParam(r, "id"), chi.URLParam(r, "action")
	if action == "maintenance_on" || action == "maintenance_off" {
		if err := h.service.SetMaintenance(r.Context(), id, action == "maintenance_on"); err != nil {
			internal(w, "set maintenance mode", err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"message": "maintenance mode updated"})
		return
	}
	task, err := h.service.CreateTask(r.Context(), id, action, "{}")
	if err != nil {
		invalid(w, err.Error())
		return
	}
	respond(w, http.StatusAccepted, task)
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.Events(r.Context(), chi.URLParam(r, "id"), intQuery(r, "limit", 100, 500))
	if err != nil {
		internal(w, "list server events", err)
		return
	}
	respond(w, http.StatusOK, items)
}
func (h *Handler) Heartbeats(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.Heartbeats(r.Context(), chi.URLParam(r, "id"), intQuery(r, "limit", 100, 500))
	if err != nil {
		internal(w, "list server heartbeats", err)
		return
	}
	respond(w, http.StatusOK, items)
}
func (h *Handler) Tasks(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.Tasks(r.Context(), chi.URLParam(r, "id"), intQuery(r, "limit", 100, 500))
	if err != nil {
		internal(w, "list server tasks", err)
		return
	}
	respond(w, http.StatusOK, items)
}

func intQuery(r *http.Request, key string, fallback, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
