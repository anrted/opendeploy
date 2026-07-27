package module

import (
	"context"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/events"
)

func (s *Service) publishEvent(ctx context.Context, eventType string, payload any) {
	event := events.NewBaseEvent(eventType, payload)
	if err := s.bus.Publish(ctx, event); err != nil {
		s.logger.Warn("module service: publish event failed", "type", eventType, "error", err)
	}
}

func (s *Service) recordAudit(ctx context.Context, userID, action, moduleID, ip string, status audit.Status) {
	resource := moduleID
	_ = s.audit.Record(ctx, audit.Entry{
		UserID: &userID, Action: action, Resource: &resource,
		IPAddress: &ip, Status: status,
	})
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
