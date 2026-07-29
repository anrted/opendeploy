// Package service implements user-managed systemd services for OpenDeploy.
//
// These are services the user explicitly adds to the panel (Redis, MySQL, etc.)
// and wants to manage from the UI. Contrast with module-owned services (like
// nginx.service, php8.3-fpm.service) which are managed by their respective modules.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// State represents the runtime state of a managed service.
type State string

const (
	StateRunning State = "running"
	StateStopped State = "stopped"
	StateFailed  State = "failed"
	StateUnknown State = "unknown"
)

// ManagedService is a systemd service tracked by OpenDeploy.
type ManagedService struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Unit        string    `json:"unit"`
	Description string    `json:"description,omitempty"`
	Autostart   bool      `json:"autostart"`
	State       State     `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ─── Repository ────────────────────────────────────────────────────────────

type Repository interface {
	Create(ctx context.Context, svc *ManagedService) error
	FindByID(ctx context.Context, id string) (*ManagedService, error)
	ListAll(ctx context.Context) ([]ManagedService, error)
	UpdateState(ctx context.Context, id string, state State) error
	Delete(ctx context.Context, id string) error
}

type sqliteRepository struct{ db *sql.DB }

func NewSQLiteRepository(db *sql.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) Create(ctx context.Context, s *ManagedService) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now
	const q = `INSERT INTO managed_services (id, name, unit, description, autostart, state, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, s.ID, s.Name, s.Unit, s.Description,
		boolInt(s.Autostart), string(s.State),
		s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339))
	return err
}

func (r *sqliteRepository) FindByID(ctx context.Context, id string) (*ManagedService, error) {
	const q = `SELECT id, name, unit, description, autostart, state, created_at, updated_at
	           FROM managed_services WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	return r.scan(row)
}

func (r *sqliteRepository) ListAll(ctx context.Context) ([]ManagedService, error) {
	const q = `SELECT id, name, unit, description, autostart, state, created_at, updated_at
	           FROM managed_services ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var svcs []ManagedService
	for rows.Next() {
		var s ManagedService
		var autostart int
		var createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Unit, &s.Description, &autostart, &s.State, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		s.Autostart = autostart == 1
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		svcs = append(svcs, s)
	}
	return svcs, rows.Err()
}

func (r *sqliteRepository) UpdateState(ctx context.Context, id string, state State) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE managed_services SET state=?, updated_at=? WHERE id=?`,
		string(state), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (r *sqliteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM managed_services WHERE id = ?`, id)
	return err
}

func (r *sqliteRepository) scan(row *sql.Row) (*ManagedService, error) {
	var s ManagedService
	var autostart int
	var createdAt, updatedAt string
	err := row.Scan(&s.ID, &s.Name, &s.Unit, &s.Description, &autostart, &s.State, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("service")
		}
		return nil, fmt.Errorf("service repo: scan: %w", err)
	}
	s.Autostart = autostart == 1
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// ─── Service (business logic) ───────────────────────────────────────────────

// SvcService manages systemd services via the Agent.
type SvcService struct {
	repo   Repository
	agent  contract.AgentClient
	logger *slog.Logger
}

// NewSvcService constructs the service manager.
func NewSvcService(repo Repository, agent contract.AgentClient, logger *slog.Logger) *SvcService {
	return &SvcService{repo: repo, agent: agent, logger: logger}
}

// List returns all managed services with live state from Agent.
func (s *SvcService) List(ctx context.Context) ([]ManagedService, error) {
	svcs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	// Enrich with live state from Agent.
	for i, svc := range svcs {
		if s.agent == nil {
			break
		}
		status, err := s.agent.ServiceStatus(ctx, svc.Unit)
		if err != nil {
			svcs[i].State = StateUnknown
			continue
		}
		if status.Active {
			svcs[i].State = StateRunning
		} else {
			svcs[i].State = StateStopped
		}
	}
	return svcs, nil
}

// Add registers a new managed service (does NOT start it).
func (s *SvcService) Add(ctx context.Context, name, unit, description string, autostart bool) (*ManagedService, error) {
	svc := &ManagedService{
		Name:        name,
		Unit:        unit,
		Description: description,
		Autostart:   autostart,
		State:       StateUnknown,
	}
	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

// Start starts a managed service.
func (s *SvcService) Start(ctx context.Context, id string) error {
	svc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return apperrors.New(503, apperrors.CodeAgentUnavailable, "agent not available")
	}
	if err := s.agent.ServiceStart(ctx, svc.Unit); err != nil {
		_ = s.repo.UpdateState(ctx, id, StateFailed)
		return apperrors.Internal("start service", err)
	}
	return s.repo.UpdateState(ctx, id, StateRunning)
}

// Stop stops a managed service.
func (s *SvcService) Stop(ctx context.Context, id string) error {
	svc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return apperrors.New(503, apperrors.CodeAgentUnavailable, "agent not available")
	}
	if err := s.agent.ServiceStop(ctx, svc.Unit); err != nil {
		return apperrors.Internal("stop service", err)
	}
	return s.repo.UpdateState(ctx, id, StateStopped)
}

// Restart restarts a managed service.
func (s *SvcService) Restart(ctx context.Context, id string) error {
	svc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return apperrors.New(503, apperrors.CodeAgentUnavailable, "agent not available")
	}
	return s.agent.ServiceRestart(ctx, svc.Unit)
}

// Logs returns the last n journal lines for a managed service.
func (s *SvcService) Logs(ctx context.Context, id string, lines int) ([]string, error) {
	svc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, apperrors.New(503, apperrors.CodeAgentUnavailable, "agent not available")
	}
	return s.agent.ServiceLogs(ctx, svc.Unit, lines)
}

// StreamLogs returns a channel that streams live logs for a managed service.
func (s *SvcService) StreamLogs(ctx context.Context, id string) (<-chan string, error) {
	svc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, apperrors.New(503, apperrors.CodeAgentUnavailable, "agent not available")
	}
	return s.agent.ServiceStreamLogs(ctx, svc.Unit)
}

// Remove deletes a managed service record (does NOT stop/uninstall the actual service).
func (s *SvcService) Remove(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ─── HTTP Handler ───────────────────────────────────────────────────────────

// Handler exposes managed services over HTTP.
type Handler struct {
	svc *SvcService
}

// NewHandler constructs a service Handler.
func NewHandler(svc *SvcService) *Handler {
	return &Handler{svc: svc}
}

type addServiceRequest struct {
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	Autostart   bool   `json:"autostart"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	svcs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, svcs)
}

func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	var req addServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON"))
		return
	}
	if req.Unit == "" {
		writeError(w, apperrors.InvalidInput("unit is required"))
		return
	}
	svc, err := h.svc.Add(r.Context(), req.Name, req.Unit, req.Description, req.Autostart)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusCreated, svc)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Start(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "service started"})
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Stop(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "service stopped"})
}

func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Restart(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "service restarted"})
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lines, err := h.svc.Logs(r.Context(), id, 100)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"lines": lines})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, err := h.svc.StreamLogs(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // upgrader already writes the error response
	}
	defer conn.Close()

	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
	}
}

func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Remove(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "service removed"})
}

// ─── helpers ───────────────────────────────────────────────────────────────

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	apperrors.WriteHTTP(w, err)
}
