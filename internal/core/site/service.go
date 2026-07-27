package site

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/internal/platform/events"
	"github.com/anrted/opendeploy/internal/platform/osprovider"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Service implements site management business logic.
type Service struct {
	repo   Repository
	audit  *audit.Service
	agent  contract.AgentClient
	logger *slog.Logger
	events events.Bus
	files  *FileService
	deploy *DeployService
}

// NewService constructs a site Service.
func NewService(repo Repository, auditSvc *audit.Service, agent contract.AgentClient, registry *module.Registry, eventBus events.Bus, logger *slog.Logger) *Service {
	return &Service{
		repo: repo, audit: auditSvc, agent: agent,
		events: eventBus, files: NewFileService(repo, agent),
		deploy: NewDeployService(registry), logger: logger,
	}
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
		Name:     req.Name,
		ModuleID: req.ModuleID,
		RootPath: rootPath,
		State:    StateActive,
		OwnerID:  &userID,
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

	// Resolve the OS web identity. There is deliberately no numeric fallback:
	// UID/GID values are host-specific and a wrong chown is a privilege bug.
	provider, err := osprovider.NewProvider()
	if err == nil {
		webAccount, lookupErr := user.Lookup(provider.WebUser())
		if lookupErr != nil {
			_ = s.agent.FileDelete(ctx, site.RootPath)
			return nil, apperrors.Internal("failed to resolve the OS web user", lookupErr)
		}
		webUID, uidErr := strconv.Atoi(webAccount.Uid)
		webGID, gidErr := strconv.Atoi(webAccount.Gid)
		if uidErr != nil || gidErr != nil {
			_ = s.agent.FileDelete(ctx, site.RootPath)
			return nil, apperrors.Internal("invalid OS web user identity", errors.Join(uidErr, gidErr))
		}
		if chownErr := s.agent.FileChown(ctx, site.RootPath, webUID, webGID); chownErr != nil {
			_ = s.agent.FileDelete(ctx, site.RootPath)
			return nil, apperrors.Internal("failed to assign site root ownership", chownErr)
		}
	} else {
		s.logger.WarnContext(ctx, "OS provider unavailable; preserving site directory ownership", "error", err)
	}

	needsCertbot := site.SSL != nil && site.SSL.Provider == "certbot"

	if needsCertbot {
		// Temporary HTTP configuration for Certbot challenge
		tmpSpec := *site
		tmpSpec.SSL = nil // Disable SSL temporarily
		if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, &tmpSpec); err != nil {
			return nil, apperrors.Internal("failed to provision temp web server for certbot", err)
		}
		time.Sleep(2 * time.Second) // wait for web server to restart
		if err := s.obtainCertbotSSL(ctx, req.Domain, "/var/www/_opendeploy_acme"); err != nil {
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

	s.publishLifecycle(ctx, EventCreated, site, userID, ip)
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
		} else if req.SSLCert != nil && strings.HasPrefix(*req.SSLCert, "/etc/letsencrypt") && site.SSL.Provider == "custom" {
			site.SSL.Provider = "certbot"
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

	needsCertbot := false
	if site.SSL != nil && site.SSL.Provider == "certbot" {
		if previous.SSL == nil || previous.SSL.Provider != "certbot" {
			needsCertbot = true
		}
	}

	if needsCertbot {
		var primaryDomain string
		for _, d := range site.Domains {
			if d.Type == DomainPrimary {
				primaryDomain = d.Domain
				break
			}
		}

		tmpSpec := *site
		tmpSpec.SSL = nil
		if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, &tmpSpec); err != nil {
			return nil, apperrors.Internal("failed to provision temp web server for certbot", err)
		}
		time.Sleep(2 * time.Second)
		if err := s.obtainCertbotSSL(ctx, primaryDomain, "/var/www/_opendeploy_acme"); err != nil {
			_ = s.applySiteConfig(ctx, previous.ModuleID, contract.SiteUpsert, &previous)
			if _, ok := err.(*apperrors.AppError); ok {
				return nil, err
			}
			return nil, apperrors.Internal("failed to obtain SSL certificate", err)
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

	s.publishLifecycle(ctx, EventUpdated, site, userID, ip)
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
	s.publishLifecycle(ctx, EventDeleted, site, userID, ip)
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
	s.publishLifecycle(ctx, EventEnabled, site, userID, ip)
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
	s.publishLifecycle(ctx, EventDisabled, site, userID, ip)
	return nil
}

// applySiteConfig passes the current domain model state to the web server module.
func (s *Service) applySiteConfig(ctx context.Context, moduleID string, action contract.SiteAction, site *Site) error {
	return s.deploy.Apply(ctx, moduleID, action, site)
}

func (s *Service) obtainCertbotSSL(ctx context.Context, domain, rootPath string) error {
	return s.deploy.ObtainCertificate(ctx, domain, rootPath)
}

// ─── helpers ───────────────────────────────────────────────────────────────

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

func (s *Service) publishLifecycle(ctx context.Context, eventType string, current *Site, actorID, ipAddress string) {
	if s.events == nil {
		return
	}
	event := newLifecycleEvent(eventType, current, actorID, ipAddress)
	if err := s.events.Publish(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "site lifecycle event delivery failed",
			"event", eventType, "event_id", event.ID(), "site_id", current.ID, "error", err)
	}
}
