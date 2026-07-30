package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/anrted/opendeploy/internal/core/servercontext"
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

type sqliteRepository struct {
	db *sql.DB
}

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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("site repo: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO sites (id, server_id, name, module_id, root_path, status, owner_id, proxy_enabled, proxy_host, proxy_port, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, servercontext.ID(ctx), s.Name, s.ModuleID, s.RootPath, string(s.State), s.OwnerID, boolInt(s.ProxyEnabled), s.ProxyHost, s.ProxyPort, s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("site repo: create site: %w", err)
	}

	for i := range s.Domains {
		if s.Domains[i].ID == "" {
			s.Domains[i].ID = uuid.New().String()
		}
		s.Domains[i].SiteID = s.ID
		s.Domains[i].CreatedAt = now
		_, err = tx.ExecContext(ctx,
			`INSERT INTO site_domains (id, site_id, server_id, domain, domain_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			s.Domains[i].ID, s.Domains[i].SiteID, servercontext.ID(ctx), s.Domains[i].Domain, string(s.Domains[i].Type), s.Domains[i].CreatedAt.Format(time.RFC3339),
		)
		if err != nil {
			if isSQLiteUniqueError(err) {
				return apperrors.New(409, apperrors.CodeSiteAlreadyExists, "domain already exists: "+s.Domains[i].Domain)
			}
			return fmt.Errorf("site repo: create domain: %w", err)
		}
	}

	s.App.SiteID = s.ID
	_, err = tx.ExecContext(ctx,
		`INSERT INTO site_apps (site_id, app_type, app_version, proxy_target, custom_config) VALUES (?, ?, ?, ?, ?)`,
		s.App.SiteID, s.App.AppType, s.App.AppVersion, s.App.ProxyTarget, s.App.CustomConfig,
	)
	if err != nil {
		return fmt.Errorf("site repo: create app: %w", err)
	}

	if s.SSL != nil {
		if s.SSL.ID == "" {
			s.SSL.ID = uuid.New().String()
		}
		s.SSL.SiteID = s.ID
		var expiresAt *string
		if s.SSL.ExpiresAt != nil {
			t := s.SSL.ExpiresAt.Format(time.RFC3339)
			expiresAt = &t
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO site_ssl (id, site_id, provider, cert_path, key_path, force_https, auto_renew, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			s.SSL.ID, s.SSL.SiteID, s.SSL.Provider, s.SSL.CertPath, s.SSL.KeyPath, boolInt(s.SSL.ForceHTTPS), boolInt(s.SSL.AutoRenew), expiresAt,
		)
		if err != nil {
			return fmt.Errorf("site repo: create ssl: %w", err)
		}
	}

	return tx.Commit()
}

func (r *sqliteRepository) FindByID(ctx context.Context, id string) (*Site, error) {
	site, err := r.findSite(ctx, `SELECT id, name, module_id, root_path, status, owner_id, proxy_enabled, proxy_host, proxy_port, created_at, updated_at FROM sites WHERE id = ? AND server_id = ?`, id, servercontext.ID(ctx))
	if err != nil {
		return nil, err
	}
	if err := r.loadRelations(ctx, site); err != nil {
		return nil, err
	}
	return site, nil
}

func (r *sqliteRepository) FindByDomain(ctx context.Context, domain string) (*Site, error) {
	var siteID string
	err := r.db.QueryRowContext(ctx, `SELECT d.site_id FROM site_domains d JOIN sites s ON s.id=d.site_id WHERE d.domain = ? AND s.server_id = ?`, domain, servercontext.ID(ctx)).Scan(&siteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("site")
		}
		return nil, fmt.Errorf("site repo: find site id by domain: %w", err)
	}
	return r.FindByID(ctx, siteID)
}

func (r *sqliteRepository) ListAll(ctx context.Context) ([]Site, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, module_id, root_path, status, owner_id, proxy_enabled, proxy_host, proxy_port, created_at, updated_at FROM sites WHERE server_id=?`, servercontext.ID(ctx))
	if err != nil {
		return nil, fmt.Errorf("site repo: list: %w", err)
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var s Site
		var created, updated string
		var proxyHost sql.NullString
		var proxyPort sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Name, &s.ModuleID, &s.RootPath, &s.State, &s.OwnerID, &s.ProxyEnabled, &proxyHost, &proxyPort, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan site %s: %w", s.ID, err)
		}
		s.ProxyHost = proxyHost.String
		s.ProxyPort = int(proxyPort.Int64)
		s.CreatedAt, _ = time.Parse(time.RFC3339, created)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		sites = append(sites, s)
	}

	for i := range sites {
		if err := r.loadRelations(ctx, &sites[i]); err != nil {
			return nil, err
		}
	}

	return sites, nil
}

func (r *sqliteRepository) Update(ctx context.Context, s *Site) error {
	s.UpdatedAt = time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE sites SET name=?, module_id=?, root_path=?, status=?, proxy_enabled=?, proxy_host=?, proxy_port=?, updated_at=? WHERE id=? AND server_id=?`,
		s.Name, s.ModuleID, s.RootPath, string(s.State), boolInt(s.ProxyEnabled), s.ProxyHost, s.ProxyPort, s.UpdatedAt.Format(time.RFC3339), s.ID, servercontext.ID(ctx),
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NotFound("site")
	}

	_, err = tx.ExecContext(ctx, `UPDATE site_apps SET app_type=?, app_version=?, proxy_target=?, custom_config=? WHERE site_id=?`,
		s.App.AppType, s.App.AppVersion, s.App.ProxyTarget, s.App.CustomConfig, s.ID,
	)
	if err != nil {
		return err
	}

	if s.SSL != nil {
		if s.SSL.ID == "" {
			s.SSL.ID = uuid.New().String()
			s.SSL.SiteID = s.ID
		}
		var expiresAt *string
		if s.SSL.ExpiresAt != nil {
			t := s.SSL.ExpiresAt.Format(time.RFC3339)
			expiresAt = &t
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO site_ssl (id, site_id, provider, cert_path, key_path, force_https, auto_renew, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, cert_path=excluded.cert_path, key_path=excluded.key_path,
			force_https=excluded.force_https, auto_renew=excluded.auto_renew, expires_at=excluded.expires_at`,
			s.SSL.ID, s.ID, s.SSL.Provider, s.SSL.CertPath, s.SSL.KeyPath, boolInt(s.SSL.ForceHTTPS), boolInt(s.SSL.AutoRenew), expiresAt,
		)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM site_ssl WHERE site_id=?`, s.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *sqliteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ? AND server_id = ?`, id, servercontext.ID(ctx))
	return err
}

func (r *sqliteRepository) findSite(ctx context.Context, query string, args ...any) (*Site, error) {
	var s Site
	var created, updated string
	var proxyHost sql.NullString
	var proxyPort sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&s.ID, &s.Name, &s.ModuleID, &s.RootPath, &s.State, &s.OwnerID, &s.ProxyEnabled, &proxyHost, &proxyPort, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("site")
		}
		return nil, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, created)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	s.ProxyHost = proxyHost.String
	s.ProxyPort = int(proxyPort.Int64)
	return &s, nil
}

func (r *sqliteRepository) loadRelations(ctx context.Context, s *Site) error {
	// load domains
	rows, err := r.db.QueryContext(ctx, `SELECT id, domain, domain_type, created_at FROM site_domains WHERE site_id=?`, s.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	s.Domains = []Domain{}
	for rows.Next() {
		var d Domain
		var created string
		if err := rows.Scan(&d.ID, &d.Domain, &d.Type, &created); err != nil {
			return err
		}
		d.SiteID = s.ID
		d.CreatedAt, _ = time.Parse(time.RFC3339, created)
		s.Domains = append(s.Domains, d)
	}

	// load app
	err = r.db.QueryRowContext(ctx, `SELECT app_type, app_version, proxy_target, custom_config FROM site_apps WHERE site_id=?`, s.ID).
		Scan(&s.App.AppType, &s.App.AppVersion, &s.App.ProxyTarget, &s.App.CustomConfig)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	s.App.SiteID = s.ID

	// load ssl
	var ssl SSL
	var expiresAt *string
	var force, renew int
	err = r.db.QueryRowContext(ctx, `SELECT id, provider, cert_path, key_path, force_https, auto_renew, expires_at FROM site_ssl WHERE site_id=?`, s.ID).
		Scan(&ssl.ID, &ssl.Provider, &ssl.CertPath, &ssl.KeyPath, &force, &renew, &expiresAt)
	if err == nil {
		ssl.SiteID = s.ID
		ssl.ForceHTTPS = force == 1
		ssl.AutoRenew = renew == 1
		if expiresAt != nil {
			t, _ := time.Parse(time.RFC3339, *expiresAt)
			ssl.ExpiresAt = &t
		}
		s.SSL = &ssl
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isSQLiteUniqueError(err error) bool {
	return err != nil && len(err.Error()) > 0 && containsStr(err.Error(), "UNIQUE constraint failed")
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
