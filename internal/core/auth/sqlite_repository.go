// Package auth — SQLite implementations of UserRepository and SessionRepository.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

// ─── sqliteUserRepository ──────────────────────────────────────────────────

type sqliteUserRepository struct {
	db *sql.DB
}

// NewSQLiteUserRepository creates a UserRepository backed by SQLite.
func NewSQLiteUserRepository(db *sql.DB) UserRepository {
	return &sqliteUserRepository{db: db}
}

func (r *sqliteUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	const q = `SELECT id, username, email, password, role, created_at, updated_at, last_login
	           FROM users WHERE id = ?`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

func (r *sqliteUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	const q = `SELECT id, username, email, password, role, created_at, updated_at, last_login
	           FROM users WHERE username = ?`
	return r.scanOne(r.db.QueryRowContext(ctx, q, username))
}

func (r *sqliteUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const q = `SELECT id, username, email, password, role, created_at, updated_at, last_login
	           FROM users WHERE email = ?`
	return r.scanOne(r.db.QueryRowContext(ctx, q, email))
}

func (r *sqliteUserRepository) Create(ctx context.Context, u *User) error {
	const q = `INSERT INTO users (id, username, email, password, role, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		u.ID, u.Username, u.Email, u.Password, string(u.Role),
		u.CreatedAt.UTC().Format(time.RFC3339),
		u.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if isSQLiteUniqueError(err) {
			return apperrors.AlreadyExists("user")
		}
		return fmt.Errorf("user repository: create: %w", err)
	}
	return nil
}

func (r *sqliteUserRepository) Update(ctx context.Context, u *User) error {
	const q = `UPDATE users SET username=?, email=?, password=?, role=?, updated_at=?, last_login=?
	           WHERE id=?`
	var lastLogin *string
	if u.LastLogin != nil {
		s := u.LastLogin.UTC().Format(time.RFC3339)
		lastLogin = &s
	}
	res, err := r.db.ExecContext(ctx, q,
		u.Username, u.Email, u.Password, string(u.Role),
		u.UpdatedAt.UTC().Format(time.RFC3339),
		lastLogin,
		u.ID,
	)
	if err != nil {
		return fmt.Errorf("user repository: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func (r *sqliteUserRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM users WHERE id = ?`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

func (r *sqliteUserRepository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *sqliteUserRepository) scanOne(row *sql.Row) (*User, error) {
	u := &User{}
	var role string
	var createdAt, updatedAt string
	var lastLogin *string

	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&role, &createdAt, &updatedAt, &lastLogin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("user repository: scan: %w", err)
	}

	u.Role = Role(role)
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastLogin != nil {
		t, _ := time.Parse(time.RFC3339, *lastLogin)
		u.LastLogin = &t
	}
	return u, nil
}

// ─── sqliteSessionRepository ───────────────────────────────────────────────

type sqliteSessionRepository struct {
	db *sql.DB
}

// NewSQLiteSessionRepository creates a SessionRepository backed by SQLite.
func NewSQLiteSessionRepository(db *sql.DB) SessionRepository {
	return &sqliteSessionRepository{db: db}
}

func (r *sqliteSessionRepository) Create(ctx context.Context, s *Session) error {
	const q = `INSERT INTO sessions (id, user_id, token_hash, ip_address, user_agent, expires_at, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		s.ID, s.UserID, s.TokenHash, s.IPAddress, s.UserAgent,
		s.ExpiresAt.UTC().Format(time.RFC3339),
		s.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("session repository: create: %w", err)
	}
	return nil
}

func (r *sqliteSessionRepository) FindByTokenHash(ctx context.Context, hash string) (*Session, error) {
	const q = `SELECT id, user_id, token_hash, ip_address, user_agent, expires_at, created_at
	           FROM sessions WHERE token_hash = ?`
	row := r.db.QueryRowContext(ctx, q, hash)

	s := &Session{}
	var expiresAt, createdAt string
	err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.IPAddress, &s.UserAgent, &expiresAt, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.New(401, apperrors.CodeSessionNotFound, "session not found")
		}
		return nil, fmt.Errorf("session repository: find by hash: %w", err)
	}
	s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return s, nil
}

func (r *sqliteSessionRepository) DeleteByID(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (r *sqliteSessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *sqliteSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// ─── helpers ───────────────────────────────────────────────────────────────

// isSQLiteUniqueError detects UNIQUE constraint violation from mattn/go-sqlite3.
func isSQLiteUniqueError(err error) bool {
	if err == nil {
		return false
	}
	return containsString(err.Error(), "UNIQUE constraint failed")
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
