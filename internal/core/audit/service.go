// Package audit implements the audit log domain for OpenDeploy.
//
// Every significant action (login, module install, site create, etc.) is
// recorded in the audit_log table. Audit entries are append-only; they are
// never modified or deleted (except via explicit retention policy).
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status represents the outcome of an audited action.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// Entry is a single record in the audit log.
type Entry struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id,omitempty"`
	Action    string    `json:"action"`             // e.g. "module.install", "site.create"
	Resource  *string   `json:"resource,omitempty"` // e.g. "nginx", "site:uuid"
	Metadata  any       `json:"metadata,omitempty"` // JSON-serialisable extra context
	IPAddress *string   `json:"ip_address,omitempty"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Service records audit entries into the database.
type Service struct {
	db *sql.DB
}

// NewService constructs an audit Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Record appends an audit entry. It is fire-and-forget — errors are returned
// but callers should not treat them as fatal (audit failure ≠ operation failure).
func (s *Service) Record(ctx context.Context, entry Entry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	var meta *string
	if entry.Metadata != nil {
		b, err := json.Marshal(entry.Metadata)
		if err == nil {
			str := string(b)
			meta = &str
		}
	}

	const q = `INSERT INTO audit_log (id, user_id, action, resource, metadata, ip_address, status, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		entry.ID, entry.UserID, entry.Action, entry.Resource,
		meta, entry.IPAddress, string(entry.Status),
		entry.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("audit: record: %w", err)
	}
	return nil
}

// List returns the most recent audit entries up to the given limit.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Entry, error) {
	const q = `SELECT id, user_id, action, resource, metadata, ip_address, status, created_at
	           FROM audit_log ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var createdAt string
		var meta *string
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Action, &e.Resource,
			&meta, &e.IPAddress, &e.Status, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("audit: scan: %w", err)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if meta != nil {
			var m any
			_ = json.Unmarshal([]byte(*meta), &m)
			e.Metadata = m
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Service) ListForUser(ctx context.Context, userID string, limit, offset int) ([]Entry, error) {
	const q = `SELECT id, user_id, action, resource, metadata, ip_address, status, created_at
	           FROM audit_log WHERE user_id = ? OR resource = ?
	           ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, userID, "user:"+userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit: list for user: %w", err)
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var createdAt string
		var meta *string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Resource, &meta, &e.IPAddress, &e.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("audit: scan for user: %w", err)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if meta != nil {
			_ = json.Unmarshal([]byte(*meta), &e.Metadata)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
