package site

import (
	"context"
	"log/slog"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/events"
)

func RegisterAuditSubscriber(bus events.Bus, auditService *audit.Service, logger *slog.Logger) events.UnsubscribeFn {
	unsubscribers := make([]events.UnsubscribeFn, 0, 5)
	for eventType := range siteAuditActions {
		currentEventType := eventType
		unsubscribers = append(unsubscribers, events.SubscribePayload(bus, currentEventType, func(ctx context.Context, data LifecycleEventData) error {
			action := siteAuditActions[currentEventType]
			resource := "site:" + data.SiteID
			var actorID, ipAddress *string
			if data.ActorID != "" {
				actorID = &data.ActorID
			}
			if data.IPAddress != "" {
				ipAddress = &data.IPAddress
			}
			if err := auditService.Record(ctx, audit.Entry{
				UserID: actorID, Action: action, Resource: &resource,
				Metadata:  map[string]any{"domain": data.PrimaryDomain, "module_id": data.ModuleID},
				IPAddress: ipAddress, Status: audit.StatusSuccess,
			}); err != nil {
				logger.ErrorContext(ctx, "site lifecycle audit subscriber failed", "event", currentEventType, "site_id", data.SiteID, "error", err)
				return err
			}
			return nil
		}))
	}
	return func() {
		for _, unsubscribe := range unsubscribers {
			unsubscribe()
		}
	}
}

var siteAuditActions = map[string]string{
	EventCreated: "site.create", EventUpdated: "site.update", EventDeleted: "site.delete",
	EventEnabled: "site.enable", EventDisabled: "site.disable",
}
