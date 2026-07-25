// Package settings implements the application settings store for OpenDeploy.
//
// Settings are stored as key-value pairs in the `settings` table.
// Keys are namespaced: "core.panel_title", "core.default_php", etc.
// Modules can also store their settings here using their ID as namespace.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/anrted/opendeploy/internal/core/updater"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

// Setting is a single key-value configuration entry.
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service manages application settings in the database.
type Service struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewService constructs a settings Service.
func NewService(db *sql.DB, logger *slog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Get returns the value for a setting key. Returns "" if not found (not an error).
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("settings: get %q: %w", key, err)
	}
	return value, nil
}

// GetWithDefault returns the value for key, or defaultVal if not set.
func (s *Service) GetWithDefault(ctx context.Context, key, defaultVal string) string {
	v, err := s.Get(ctx, key)
	if err != nil || v == "" {
		return defaultVal
	}
	return v
}

// Set upserts a setting.
func (s *Service) Set(ctx context.Context, key, value string) error {
	const q = `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
	           ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`
	_, err := s.db.ExecContext(ctx, q, key, value, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	return nil
}

// SetMany upserts multiple settings in a single transaction.
func (s *Service) SetMany(ctx context.Context, kv map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("settings: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	const q = `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
	           ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`
	now := time.Now().UTC().Format(time.RFC3339)
	for k, v := range kv {
		if _, err := tx.ExecContext(ctx, q, k, v, now); err != nil {
			return fmt.Errorf("settings: set %q: %w", k, err)
		}
	}
	return tx.Commit()
}

// ListByNamespace returns all settings with the given namespace prefix.
// e.g. Namespace("core") returns all keys starting with "core.".
func (s *Service) ListByNamespace(ctx context.Context, namespace string) ([]Setting, error) {
	prefix := namespace + "."
	const q = `SELECT key, value, updated_at FROM settings WHERE key LIKE ? ORDER BY key`
	rows, err := s.db.QueryContext(ctx, q, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("settings: list namespace %q: %w", namespace, err)
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var st Setting
		var updatedAt string
		if err := rows.Scan(&st.Key, &st.Value, &updatedAt); err != nil {
			return nil, err
		}
		st.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		settings = append(settings, st)
	}
	return settings, rows.Err()
}

// Delete removes a setting.
func (s *Service) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}

// ─── HTTP Handler ───────────────────────────────────────────────────────────

// Handler exposes settings over HTTP.
type Handler struct {
	svc     *Service
	updates *updater.Service
}

// NewHandler constructs a settings Handler.
func NewHandler(svc *Service, updates *updater.Service) *Handler {
	return &Handler{svc: svc, updates: updates}
}

// List handles GET /api/v1/settings?ns=core
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	if ns == "" {
		ns = "core"
	}
	settings, err := h.svc.ListByNamespace(r.Context(), ns)
	if err != nil {
		writeError(w, apperrors.Internal("list settings", err))
		return
	}
	respond(w, http.StatusOK, settings)
}

// Update handles PUT /api/v1/settings — bulk update a map of key→value.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var kv map[string]string
	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	if len(kv) == 0 {
		writeError(w, apperrors.InvalidInput("no settings provided"))
		return
	}
	if err := h.svc.SetMany(r.Context(), kv); err != nil {
		writeError(w, apperrors.Internal("update settings", err))
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "settings updated"})
}

// UpdateStatus handles GET /api/v1/updates.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.updates.Check(r.Context())
	if err != nil {
		writeError(w, apperrors.Internal("check for updates", err))
		return
	}
	respond(w, http.StatusOK, status)
}

// ─── helpers ───────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	ae := apperrors.AsAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": ae.Code, "message": ae.Message},
	})
}
