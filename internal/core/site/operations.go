package site

import (
	"context"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Configuration and lifecycle collaborators are isolated from CRUD
// orchestration while the Service API remains unchanged.
func (s *Service) applySiteConfig(ctx context.Context, moduleID string, action contract.SiteAction, site *Site) error {
	return s.deploy.Apply(ctx, moduleID, action, site)
}

func (s *Service) obtainCertbotSSL(ctx context.Context, domain, rootPath string) error {
	return s.deploy.ObtainCertificate(ctx, domain, rootPath)
}

func (s *Service) recordAudit(ctx context.Context, userID, action, resource, ip string, status audit.Status) {
	if s.audit == nil {
		return
	}
	uid, res, ipAddress := &userID, resource, &ip
	_ = s.audit.Record(ctx, audit.Entry{
		UserID: uid, Action: action, Resource: &res, IPAddress: ipAddress, Status: status,
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
