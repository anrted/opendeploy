package site

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Service implements site management business logic.
type Service struct {
	repo   Repository
	audit  *audit.Service
	agent  contract.AgentClient
	logger *slog.Logger
}

// NewService constructs a site Service.
func NewService(repo Repository, auditSvc *audit.Service, agent contract.AgentClient, logger *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditSvc, agent: agent, logger: logger}
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
		req.ModuleID = "nginx"
	}
	if req.ModuleID != "nginx" {
		return nil, apperrors.InvalidInput("module_id must be nginx")
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

	if err := s.applyNginx(ctx, contract.NginxSiteUpsert, site); err != nil {
		s.recordAudit(ctx, userID, "site.create", site.Domain, ip, audit.StatusError)
		return nil, fmt.Errorf("site service: provision nginx: %w", err)
	}
	if err := s.repo.Create(ctx, site); err != nil {
		_ = s.applyNginx(ctx, contract.NginxSiteDelete, site)
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

	if err := s.applyNginx(ctx, contract.NginxSiteUpsert, site); err != nil {
		s.recordAudit(ctx, userID, "site.update", site.Domain, ip, audit.StatusError)
		return nil, fmt.Errorf("site service: apply nginx update: %w", err)
	}
	if err := s.repo.Update(ctx, site); err != nil {
		_ = s.applyNginx(ctx, contract.NginxSiteUpsert, &previous)
		if previous.Domain != site.Domain {
			_ = s.applyNginx(ctx, contract.NginxSiteDelete, site)
		}
		return nil, err
	}
	if previous.Domain != site.Domain {
		if err := s.applyNginx(ctx, contract.NginxSiteDelete, &previous); err != nil {
			s.logger.ErrorContext(ctx, "site: old nginx vhost cleanup failed", "domain", previous.Domain, "error", err)
			return nil, fmt.Errorf("site service: remove previous nginx vhost: %w", err)
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
	if err := s.applyNginx(ctx, contract.NginxSiteDelete, site); err != nil {
		s.recordAudit(ctx, userID, "site.delete", site.Domain, ip, audit.StatusError)
		return fmt.Errorf("site service: remove nginx vhost: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		_ = s.applyNginx(ctx, contract.NginxSiteUpsert, site)
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
	agentAction := contract.NginxSiteDisable
	rollbackAction := contract.NginxSiteEnable
	if state == StateActive {
		agentAction = contract.NginxSiteEnable
		rollbackAction = contract.NginxSiteDisable
	}
	if err := s.applyNginx(ctx, agentAction, site); err != nil {
		s.recordAudit(ctx, userID, action, site.Domain, ip, audit.StatusError)
		return fmt.Errorf("site service: change nginx state: %w", err)
	}
	site.State = state
	if err := s.repo.Update(ctx, site); err != nil {
		_ = s.applyNginx(ctx, rollbackAction, site)
		return err
	}
	s.recordAudit(ctx, userID, action, site.Domain, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) applyNginx(ctx context.Context, action contract.NginxSiteAction, site *Site) error {
	if s.agent == nil {
		return fmt.Errorf("nginx agent is unavailable")
	}
	spec := contract.NginxSiteSpec{
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
	return s.agent.NginxSiteApply(ctx, action, spec)
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
