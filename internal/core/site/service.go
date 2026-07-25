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
	if err := validatePHPVersion(req.PHPVersion); err != nil {
		return nil, err
	}
	if req.ModuleID == "" {
		return nil, apperrors.InvalidInput("module_id is required")
	}
	if err := validateSSL(req.SSLEnabled, req.SSLCert, req.SSLKey); err != nil {
		return nil, err
	}

	site := &Site{
		Domain:     strings.ToLower(strings.TrimSpace(req.Domain)),
		RootPath:   rootPath,
		PHPVersion: req.PHPVersion,
		SSLEnabled: req.SSLEnabled,
		SSLCert:    req.SSLCert,
		SSLKey:     req.SSLKey,
		ModuleID:   req.ModuleID,
		State:      StateActive,
		CreatedBy:  &userID,
	}

	if err := s.agent.DirCreate(ctx, site.RootPath, 0o755); err != nil {
		return nil, apperrors.Internal("failed to create site root directory", err)
	}

	needsCertbot := site.SSLEnabled && site.SSLCert != nil && strings.HasPrefix(*site.SSLCert, "/etc/letsencrypt")

	if needsCertbot {
		// Temporary HTTP configuration for Certbot challenge
		tmpSpec := *site
		tmpSpec.SSLEnabled = false
		if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, &tmpSpec); err != nil {
			return nil, apperrors.Internal("failed to provision temp web server for certbot", err)
		}
		time.Sleep(2 * time.Second) // wait for web server to restart
		if err := s.obtainCertbotSSL(ctx, site.Domain, site.RootPath); err != nil {
			_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site)
			// Return the error directly if it's already an AppError
			if _, ok := err.(*apperrors.AppError); ok {
				return nil, err
			}
			return nil, apperrors.Internal("failed to obtain SSL certificate", err)
		}
	}

	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site); err != nil {
		s.recordAudit(ctx, userID, "site.create", site.Domain, ip, audit.StatusError)
		return nil, fmt.Errorf("site service: provision web server: %w", err)
	}
	if err := s.repo.Create(ctx, site); err != nil {
		_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site)
		return nil, err
	}

	s.recordAudit(ctx, userID, "site.create", site.Domain, ip, audit.StatusSuccess)
	s.logger.InfoContext(ctx, "site: created", "id", site.ID, "domain", site.Domain)
	return site, nil
}

// Update applies partial updates to a site.
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest, userID, ip string) (*Site, error) {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previous := *site

	if req.Domain != nil {
		if err := validateDomain(*req.Domain); err != nil {
			return nil, err
		}
		site.Domain = strings.ToLower(strings.TrimSpace(*req.Domain))
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
	if req.PHPVersion != nil {
		if err := validatePHPVersion(req.PHPVersion); err != nil {
			return nil, err
		}
		site.PHPVersion = req.PHPVersion
	}
	if req.SSLEnabled != nil {
		site.SSLEnabled = *req.SSLEnabled
	}
	if req.SSLCert != nil {
		site.SSLCert = req.SSLCert
	}
	if req.SSLKey != nil {
		site.SSLKey = req.SSLKey
	}
	if err := validateSSL(site.SSLEnabled, site.SSLCert, site.SSLKey); err != nil {
		return nil, err
	}

	needsCertbot := site.SSLEnabled && !previous.SSLEnabled && site.SSLCert != nil && strings.HasPrefix(*site.SSLCert, "/etc/letsencrypt")

	if needsCertbot {
		tmpSpec := *site
		tmpSpec.SSLEnabled = false
		if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, &tmpSpec); err != nil {
			return nil, apperrors.Internal("failed to provision temp web server for certbot", err)
		}
		time.Sleep(2 * time.Second) // wait for web server to restart
		if err := s.obtainCertbotSSL(ctx, site.Domain, site.RootPath); err != nil {
			_ = s.applySiteConfig(ctx, previous.ModuleID, contract.SiteUpsert, &previous)
			if _, ok := err.(*apperrors.AppError); ok {
				return nil, err
			}
			return nil, apperrors.Internal("failed to obtain SSL certificate", err)
		}
	}

	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site); err != nil {
		s.recordAudit(ctx, userID, "site.update", site.Domain, ip, audit.StatusError)
		return nil, fmt.Errorf("site service: apply config update: %w", err)
	}
	if err := s.repo.Update(ctx, site); err != nil {
		_ = s.applySiteConfig(ctx, previous.ModuleID, contract.SiteUpsert, &previous)
		if previous.Domain != site.Domain {
			_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site)
		}
		return nil, err
	}
	if previous.Domain != site.Domain {
		if err := s.applySiteConfig(ctx, previous.ModuleID, contract.SiteDelete, &previous); err != nil {
			s.logger.ErrorContext(ctx, "site: old vhost cleanup failed", "domain", previous.Domain, "error", err)
			return nil, fmt.Errorf("site service: remove previous vhost: %w", err)
		}
	}

	s.recordAudit(ctx, userID, "site.update", site.Domain, ip, audit.StatusSuccess)
	return site, nil
}

// Delete removes a site record (does NOT remove the vhost config on disk —
// that is done by the nginx module handler).
func (s *Service) Delete(ctx context.Context, id, userID, ip string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site); err != nil {
		s.recordAudit(ctx, userID, "site.delete", site.Domain, ip, audit.StatusError)
		return fmt.Errorf("site service: remove vhost: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site)
		return fmt.Errorf("site service: delete: %w", err)
	}
	s.recordAudit(ctx, userID, "site.delete", site.Domain, ip, audit.StatusSuccess)
	s.logger.InfoContext(ctx, "site: deleted", "id", id, "domain", site.Domain)
	return nil
}

// Enable re-activates a disabled site.
func (s *Service) Enable(ctx context.Context, id, userID, ip string) error {
	return s.setState(ctx, id, StateActive, userID, ip, "site.enable")
}

// Disable disables an active site.
func (s *Service) Disable(ctx context.Context, id, userID, ip string) error {
	return s.setState(ctx, id, StateDisabled, userID, ip, "site.disable")
}

func (s *Service) setState(ctx context.Context, id string, state State, userID, ip, action string) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	agentAction := contract.SiteDisable
	rollbackAction := contract.SiteEnable
	if state == StateActive {
		agentAction = contract.SiteEnable
		rollbackAction = contract.SiteDisable
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, agentAction, site); err != nil {
		s.recordAudit(ctx, userID, action, site.Domain, ip, audit.StatusError)
		return fmt.Errorf("site service: change state: %w", err)
	}
	site.State = state
	if err := s.repo.Update(ctx, site); err != nil {
		_ = s.applySiteConfig(ctx, site.ModuleID, rollbackAction, site)
		return err
	}
	s.recordAudit(ctx, userID, action, site.Domain, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) applySiteConfig(ctx context.Context, moduleID string, action contract.SiteAction, site *Site) error {
	if s.registry == nil {
		return fmt.Errorf("module registry is unavailable")
	}
	mod := s.registry.Find(moduleID)
	if mod == nil {
		return fmt.Errorf("web server module %q not found", moduleID)
	}
	plugin, ok := mod.(contract.WebServerPlugin)
	if !ok {
		return fmt.Errorf("module %q is not a web server plugin", moduleID)
	}

	spec := contract.SiteSpec{
		Domain:     site.Domain,
		RootPath:   site.RootPath,
		SSLEnabled: site.SSLEnabled,
	}
	if site.PHPVersion != nil {
		spec.PHPVersion = *site.PHPVersion
	}
	if site.SSLCert != nil {
		spec.SSLCert = *site.SSLCert
	}
	if site.SSLKey != nil {
		spec.SSLKey = *site.SSLKey
	}
	return plugin.ApplySite(ctx, action, spec)
}

func (s *Service) obtainCertbotSSL(ctx context.Context, domain, rootPath string) error {
	if s.registry == nil {
		return apperrors.Internal("module registry is unavailable", nil)
	}
	mod := s.registry.Find("certbot")
	if mod == nil {
		return apperrors.InvalidInput("certbot module is not installed")
	}
	plugin, ok := mod.(contract.CertbotPlugin)
	if !ok {
		return apperrors.Internal("certbot module is invalid", nil)
	}
	return plugin.ObtainCert(ctx, domain, rootPath)
}

// ─── File Operations ───────────────────────────────────────────────────────

func (s *Service) resolveFilePath(ctx context.Context, siteID, relativePath string) (string, error) {
	site, err := s.repo.FindByID(ctx, siteID)
	if err != nil {
		return "", err
	}
	// Prevent path traversal.
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

// ─── helpers ───────────────────────────────────────────────────────────────

// validateDomain checks that domain is a non-empty, syntactically valid hostname.
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
