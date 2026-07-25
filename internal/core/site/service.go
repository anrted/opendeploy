package site

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Service implements site management business logic.
type Service struct {
	repo     Repository
	audit    *audit.Service
	agent    contract.AgentClient
	registry *module.Registry
	logger   *slog.Logger
}

// NewService constructs a site Service.
func NewService(repo Repository, auditSvc *audit.Service, agent contract.AgentClient, registry *module.Registry, logger *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditSvc, agent: agent, registry: registry, logger: logger}
}

// List returns all sites.
func (s *Service) List(ctx context.Context) ([]Site, error) {
	sites, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("site service: list: %w", err)
	}
	return sites, nil
}

// Get returns a single site by ID.
func (s *Service) Get(ctx context.Context, id string) (*Site, error) {
	return s.repo.FindByID(ctx, id)
}

// Create validates and persists a new site.
func (s *Service) Create(ctx context.Context, req CreateRequest, userID, ip string) (*Site, error) {
	// Validate inputs.
	if err := validateDomain(req.Domain); err != nil {
		return nil, err
	}
	rootPath, err := normalizeRootPath(req.RootPath)
	if err != nil {
		return nil, err
	}
	if err := validatePHPVersion(req.AppVersion); req.AppType == "php" && err != nil {
		return nil, err
	}
	if req.ModuleID == "" {
		return nil, apperrors.InvalidInput("module_id is required")
	}
	if err := validateSSL(req.SSLEnabled, req.SSLCert, req.SSLKey); err != nil {
		return nil, err
	}

	site := &Site{
		Name:      req.Name,
		ModuleID:  req.ModuleID,
		RootPath:  rootPath,
		State:     StateActive,
		OwnerID:   &userID,
		Domains: []Domain{
			{Domain: strings.ToLower(strings.TrimSpace(req.Domain)), Type: DomainPrimary},
		},
		App: App{
			AppType:     req.AppType,
			AppVersion:  req.AppVersion,
			ProxyTarget: req.ProxyTarget,
		},
	}

	if req.SSLEnabled {
		provider := "custom"
		if req.SSLProvider != nil && *req.SSLProvider != "" {
			provider = *req.SSLProvider
		} else if req.SSLCert != nil && strings.HasPrefix(*req.SSLCert, "/etc/letsencrypt") {
			provider = "certbot"
		}
		
		site.SSL = &SSL{
			Provider:   provider,
			CertPath:   req.SSLCert,
			KeyPath:    req.SSLKey,
			ForceHTTPS: false,
			AutoRenew:  true,
		}
	}

	if err := s.agent.DirCreate(ctx, site.RootPath, 0o755); err != nil {
		return nil, apperrors.Internal("failed to create site root directory", err)
	}
	// Temporarily hardcode UID/GID 33 (www-data on Debian) so PHP scripts can write to the directory.
	// In the future, this should use per-site isolated Linux users.
	_ = s.agent.FileChown(ctx, site.RootPath, 33, 33)

	needsCertbot := site.SSL != nil && site.SSL.Provider == "certbot"

	if needsCertbot {
		// Temporary HTTP configuration for Certbot challenge
		tmpSpec := *site
		tmpSpec.SSL = nil // Disable SSL temporarily
		if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, &tmpSpec); err != nil {
			return nil, apperrors.Internal("failed to provision temp web server for certbot", err)
		}
		time.Sleep(2 * time.Second) // wait for web server to restart
		if err := s.obtainCertbotSSL(ctx, req.Domain, site.RootPath); err != nil {
			_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site)
			// Return the error directly if it's already an AppError
			if _, ok := err.(*apperrors.AppError); ok {
				return nil, err
			}
			return nil, apperrors.Internal("failed to obtain SSL certificate", err)
		}
	}

	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site); err != nil {
		s.recordAudit(ctx, userID, "site.create", req.Domain, ip, audit.StatusError)
		return nil, fmt.Errorf("site service: provision web server: %w", err)
	}
	if err := s.repo.Create(ctx, site); err != nil {
		_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site)
		return nil, err
	}

	s.recordAudit(ctx, userID, "site.create", req.Domain, ip, audit.StatusSuccess)
	s.logger.InfoContext(ctx, "site: created", "id", site.ID, "domain", req.Domain)
	return site, nil
}

// Update applies partial updates to a site.
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest, userID, ip string) (*Site, error) {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previous := *site

	if req.Name != nil {
		site.Name = *req.Name
	}
	if req.RootPath != nil {
		rootPath, err := normalizeRootPath(*req.RootPath)
		if err != nil {
			return nil, err
		}
		if rootPath != previous.RootPath {
			if err := s.agent.DirCreate(ctx, rootPath, 0o755); err != nil {
				return nil, apperrors.Internal("failed to create new site root directory", err)
			}
		}
		site.RootPath = rootPath
	}
	if req.AppType != nil {
		site.App.AppType = *req.AppType
	}
	if req.AppVersion != nil {
		if req.AppType != nil && *req.AppType == "php" {
			if err := validatePHPVersion(req.AppVersion); err != nil {
				return nil, err
			}
		}
		site.App.AppVersion = req.AppVersion
	}
	if req.ProxyTarget != nil {
		site.App.ProxyTarget = req.ProxyTarget
	}

	if req.SSLEnabled != nil {
		if *req.SSLEnabled {
			if site.SSL == nil {
				site.SSL = &SSL{
					Provider:  "custom",
					AutoRenew: true,
				}
			}
		} else {
			site.SSL = nil
		}
	}
	if site.SSL != nil {
		if req.SSLProvider != nil {
			site.SSL.Provider = *req.SSLProvider
		}
		if req.SSLCert != nil {
			clean, err := normalizeCertificatePath(*req.SSLCert)
			if err != nil {
				return nil, apperrors.InvalidInput("ssl_cert must be an absolute path below /etc/letsencrypt or /var/lib/opendeploy")
			}
			site.SSL.CertPath = &clean
		}
		if req.SSLKey != nil {
			clean, err := normalizeCertificatePath(*req.SSLKey)
			if err != nil {
				return nil, apperrors.InvalidInput("ssl_key must be an absolute path below /etc/letsencrypt or /var/lib/opendeploy")
			}
			site.SSL.KeyPath = &clean
		}
		if req.ForceHTTPS != nil {
			site.SSL.ForceHTTPS = *req.ForceHTTPS
		}
	}

	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site); err != nil {
		return nil, fmt.Errorf("site service: re-provision web server: %w", err)
	}

	if err := s.repo.Update(ctx, site); err != nil {
		// Rollback web config on DB failure
		_ = s.applySiteConfig(ctx, previous.ModuleID, contract.SiteUpsert, &previous)
		return nil, err
	}

	s.recordAudit(ctx, userID, "site.update", site.ID, ip, audit.StatusSuccess)
	return site, nil
}

// Delete removes a site entirely.
func (s *Service) Delete(ctx context.Context, id string, userID, ip string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site); err != nil {
		s.logger.ErrorContext(ctx, "failed to remove web server config during site deletion", "error", err, "site_id", id)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.recordAudit(ctx, userID, "site.delete", site.ID, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) Enable(ctx context.Context, id string, userID, ip string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	site.State = StateActive
	if err := s.repo.Update(ctx, site); err != nil {
		return err
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteEnable, site); err != nil {
		s.logger.ErrorContext(ctx, "failed to enable web server config", "error", err, "site_id", id)
	}
	s.recordAudit(ctx, userID, "site.enable", site.ID, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) Disable(ctx context.Context, id string, userID, ip string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	site.State = StateDisabled
	if err := s.repo.Update(ctx, site); err != nil {
		return err
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteDisable, site); err != nil {
		s.logger.ErrorContext(ctx, "failed to disable web server config", "error", err, "site_id", id)
	}
	s.recordAudit(ctx, userID, "site.disable", site.ID, ip, audit.StatusSuccess)
	return nil
}

// applySiteConfig passes the current domain model state to the web server module.
func (s *Service) applySiteConfig(ctx context.Context, moduleID string, action contract.SiteAction, site *Site) error {
	m := s.registry.Find(moduleID)
	if m == nil {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	plug, ok := m.(contract.WebServerPlugin)
	if !ok {
		return fmt.Errorf("module %s does not implement WebServerPlugin", moduleID)
	}

	var primary string
	var aliases []string
	for _, d := range site.Domains {
		if d.Type == DomainPrimary {
			primary = d.Domain
		} else {
			aliases = append(aliases, d.Domain)
		}
	}

	var appVer, proxy string
	if site.App.AppVersion != nil {
		appVer = *site.App.AppVersion
	}
	if site.App.ProxyTarget != nil {
		proxy = *site.App.ProxyTarget
	}

	sslEnabled := site.SSL != nil
	var cert, key string
	var force bool
	if sslEnabled {
		if site.SSL.CertPath != nil {
			cert = *site.SSL.CertPath
		}
		if site.SSL.KeyPath != nil {
			key = *site.SSL.KeyPath
		}
		force = site.SSL.ForceHTTPS
	}

	spec := contract.SiteSpec{
		ID:            site.ID,
		Name:          site.Name,
		PrimaryDomain: primary,
		Aliases:       aliases,
		RootPath:      site.RootPath,
		AppType:       site.App.AppType,
		AppVersion:    appVer,
		ProxyTarget:   proxy,
		SSLEnabled:    sslEnabled,
		SSLCert:       cert,
		SSLKey:        key,
		ForceHTTPS:    force,
	}

	return plug.ApplySite(ctx, action, spec)
}

func (s *Service) obtainCertbotSSL(ctx context.Context, domain, rootPath string) error {
	m := s.registry.Find("certbot")
	if m == nil {
		return apperrors.Internal("certbot module is not installed", nil)
	}
	plug, ok := m.(contract.CertbotPlugin)
	if !ok {
		return apperrors.Internal("certbot module does not implement CertbotPlugin", nil)
	}

	// Wait, certbot needs domains. In Phase 1 we just pass the primary domain to CertbotPlugin.Obtain
	return plug.ObtainCert(ctx, domain, rootPath)
}

// ─── File Management Methods ───────────────────────────────────────────────

func (s *Service) resolveFilePath(ctx context.Context, siteID, relativePath string) (string, error) {
	site, err := s.repo.FindByID(ctx, siteID)
	if err != nil {
		return "", err
	}
	cleanRelative := path.Clean("/" + relativePath)
	absPath := path.Join(site.RootPath, cleanRelative)

	if !strings.HasPrefix(absPath, site.RootPath) {
		return "", apperrors.InvalidInput("invalid file path")
	}
	return absPath, nil
}

func (s *Service) ListFiles(ctx context.Context, siteID, relativePath string) ([]contract.FileInfo, error) {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, fmt.Errorf("agent is unavailable")
	}
	return s.agent.DirList(ctx, absPath)
}

func (s *Service) ReadFile(ctx context.Context, siteID, relativePath string) ([]byte, error) {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileRead(ctx, absPath)
}

func (s *Service) WriteFile(ctx context.Context, siteID, relativePath string, content []byte) error {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	// Use 0644 for files.
	return s.agent.FileWrite(ctx, absPath, content, 0o644)
}

func (s *Service) CreateDirectory(ctx context.Context, siteID, relativePath string) error {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.DirCreate(ctx, absPath, 0o755)
}

func (s *Service) DeleteFile(ctx context.Context, siteID, relativePath string) error {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	// Protect the root directory itself.
	site, _ := s.repo.FindByID(ctx, siteID)
	if absPath == site.RootPath {
		return apperrors.InvalidInput("cannot delete site root directory")
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileDelete(ctx, absPath)
}

func (s *Service) RenameFile(ctx context.Context, siteID, oldPath, newPath string) error {
	absOld, err := s.resolveFilePath(ctx, siteID, oldPath)
	if err != nil {
		return err
	}
	absNew, err := s.resolveFilePath(ctx, siteID, newPath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileRename(ctx, absOld, absNew)
}

func (s *Service) CopyFile(ctx context.Context, siteID, srcPath, dstPath string) error {
	absSrc, err := s.resolveFilePath(ctx, siteID, srcPath)
	if err != nil {
		return err
	}
	absDst, err := s.resolveFilePath(ctx, siteID, dstPath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileCopy(ctx, absSrc, absDst)
}

func (s *Service) ChmodFile(ctx context.Context, siteID, relativePath string, mode uint32) error {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileChmod(ctx, absPath, mode)
}

func (s *Service) ChownFile(ctx context.Context, siteID, relativePath string, uid, gid int) error {
	absPath, err := s.resolveFilePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileChown(ctx, absPath, uid, gid)
}


// ─── helpers ───────────────────────────────────────────────────────────────

func validateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return apperrors.InvalidInput("domain is required")
	}
	if len(domain) > 253 {
		return apperrors.InvalidInput("domain must be ≤ 253 characters")
	}
	if strings.HasSuffix(domain, ".") {
		domain = strings.TrimSuffix(domain, ".")
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return apperrors.InvalidInput("domain contains an invalid DNS label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return apperrors.InvalidInput("domain labels cannot start or end with a hyphen")
		}
		for _, ch := range label {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') || ch == '-') {
				return apperrors.InvalidInput(fmt.Sprintf("domain contains invalid character: %q", ch))
			}
		}
	}
	return nil
}

func normalizeRootPath(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", apperrors.InvalidInput("root_path is required")
	}
	if strings.ContainsRune(root, '\\') || !strings.HasPrefix(root, "/") {
		return "", apperrors.InvalidInput("root_path must be an absolute Linux path")
	}
	clean := path.Clean(root)
	if !isSafeLinuxPath(clean) {
		return "", apperrors.InvalidInput("root_path contains unsupported characters")
	}
	allowed := false
	for _, prefix := range []string{"/var/www/", "/srv/"} {
		if clean != strings.TrimSuffix(prefix, "/") && strings.HasPrefix(clean+"/", prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", apperrors.InvalidInput("root_path must be located below /var/www or /srv")
	}
	return clean, nil
}

func validatePHPVersion(version *string) error {
	if version == nil {
		return nil
	}
	value := strings.TrimSpace(*version)
	parts := strings.Split(value, ".")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) {
		return apperrors.InvalidInput("php_version must use major.minor format")
	}
	*version = value
	return nil
}

func validateSSL(enabled bool, cert, key *string) error {
	if !enabled {
		return nil
	}
	if cert == nil || key == nil || strings.TrimSpace(*cert) == "" || strings.TrimSpace(*key) == "" {
		return apperrors.InvalidInput("ssl_cert and ssl_key are required when SSL is enabled")
	}
	for name, value := range map[string]string{"ssl_cert": *cert, "ssl_key": *key} {
		clean, err := normalizeCertificatePath(value)
		if err != nil {
			return apperrors.InvalidInput(name + " must be an absolute path below /etc/letsencrypt or /var/lib/opendeploy")
		}
		if name == "ssl_cert" {
			*cert = clean
		} else {
			*key = clean
		}
	}
	return nil
}

func normalizeCertificatePath(value string) (string, error) {
	if strings.ContainsRune(value, '\\') || !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("not an absolute Linux path")
	}
	clean := path.Clean(strings.TrimSpace(value))
	if !isSafeLinuxPath(clean) {
		return "", fmt.Errorf("path contains unsupported characters")
	}
	for _, prefix := range []string{"/etc/letsencrypt/", "/var/lib/opendeploy/"} {
		if strings.HasPrefix(clean, prefix) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("path is outside managed certificate roots")
}

func isSafeLinuxPath(value string) bool {
	if value == "" || value[0] != '/' {
		return false
	}
	for _, ch := range value {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '/' || ch == '.' ||
			ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (s *Service) recordAudit(ctx context.Context, userID, action, resource, ip string, status audit.Status) {
	if s.audit == nil {
		return
	}
	uid := &userID
	res := resource
	ipPtr := &ip
	_ = s.audit.Record(ctx, audit.Entry{
		UserID:    uid,
		Action:    action,
		Resource:  &res,
		IPAddress: ipPtr,
		Status:    status,
	})
}
