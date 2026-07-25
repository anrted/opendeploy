package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/google/uuid"
)

// Repository defines persistence for Site entities.
type Repository interface {
	Create(ctx context.Context, site *Site) error
	FindByID(ctx context.Context, id string) (*Site, error)
	FindByDomain(ctx context.Context, domain string) (*Site, error)
	ListAll(ctx context.Context) ([]Site, error)
	Update(ctx context.Context, site *Site) error
	Delete(ctx context.Context, id string) error
}

// sqliteRepository is the SQLite implementation.
type sqliteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a site Repository backed by SQLite.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) Create(ctx context.Context, s *Site) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now

	const q = `INSERT INTO sites
	           (id, domain, root_path, php_version, ssl_enabled, ssl_cert, ssl_key,
	            module_id, state, created_by, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		s.ID, s.Domain, s.RootPath, s.PHPVersion,
		boolInt(s.SSLEnabled), s.SSLCert, s.SSLKey,
		s.ModuleID, string(s.State), s.CreatedBy,
		s.CreatedAt.Format(time.RFC3339),
		s.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isSQLiteUniqueError(err) {
			return apperrors.New(409, apperrors.CodeSiteAlreadyExists, "domain already exists: "+s.Domain)
		}
		return fmt.Errorf("site repo: create: %w", err)
	}
	return nil
}

func (r *sqliteRepository) FindByID(ctx context.Context, id string) (*Site, error) {
	const q = `SELECT id, domain, root_path, php_version, ssl_enabled, ssl_cert, ssl_key,
	           module_id, state, created_by, created_at, updated_at
	           FROM sites WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	s, err := r.scan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("site")
		}
		return nil, fmt.Errorf("site repo: find by id: %w", err)
	}
	return s, nil
}

func (r *sqliteRepository) FindByDomain(ctx context.Context, domain string) (*Site, error) {
	const q = `SELECT id, domain, root_path, php_version, ssl_enabled, ssl_cert, ssl_key,
	           module_id, state, created_by, created_at, updated_at
	           FROM sites WHERE domain = ?`
	row := r.db.QueryRowContext(ctx, q, domain)
	s, err := r.scan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("site")
		}
		return nil, fmt.Errorf("site repo: find by domain: %w", err)
	}
	return s, nil
}

func (r *sqliteRepository) ListAll(ctx context.Context) ([]Site, error) {
	const q = `SELECT id, domain, root_path, php_version, ssl_enabled, ssl_cert, ssl_key,
	           module_id, state, created_by, created_at, updated_at
	           FROM sites ORDER BY domain`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("site repo: list: %w", err)
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var s Site
		var sslEnabled int
		var phpVersion, sslCert, sslKey, createdBy *string
		var createdAt, updatedAt string
		if err := rows.Scan(
			&s.ID, &s.Domain, &s.RootPath, &phpVersion, &sslEnabled,
			&sslCert, &sslKey, &s.ModuleID, &s.State, &createdBy,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("site repo: scan: %w", err)
		}
		s.SSLEnabled = sslEnabled == 1
		s.PHPVersion = phpVersion
		s.SSLCert = sslCert
		s.SSLKey = sslKey
		s.CreatedBy = createdBy
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		sites = append(sites, s)
	}
	return sites, rows.Err()
}

func (r *sqliteRepository) Update(ctx context.Context, s *Site) error {
	s.UpdatedAt = time.Now().UTC()
	const q = `UPDATE sites SET domain=?, root_path=?, php_version=?, ssl_enabled=?,
	           ssl_cert=?, ssl_key=?, module_id=?, state=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, q,
		s.Domain, s.RootPath, s.PHPVersion, boolInt(s.SSLEnabled),
		s.SSLCert, s.SSLKey, s.ModuleID, string(s.State),
		s.UpdatedAt.Format(time.RFC3339), s.ID,
	)
	if err != nil {
		return fmt.Errorf("site repo: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperrors.NotFound("site")
	}
	return nil
}

func (r *sqliteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	return err
}

// scan reads a single row into a Site.
func (r *sqliteRepository) scan(row *sql.Row) (*Site, error) {
	var s Site
	var sslEnabled int
	var phpVersion, sslCert, sslKey, createdBy *string
	var createdAt, updatedAt string
	err := row.Scan(
		&s.ID, &s.Domain, &s.RootPath, &phpVersion, &sslEnabled,
		&sslCert, &sslKey, &s.ModuleID, &s.State, &createdBy,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.SSLEnabled = sslEnabled == 1
	s.PHPVersion = phpVersion
	s.SSLCert = sslCert
	s.SSLKey = sslKey
	s.CreatedBy = createdBy
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isSQLiteUniqueError(err error) bool {
	return err != nil && len(err.Error()) > 0 &&
		containsStr(err.Error(), "UNIQUE constraint failed")
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
