package dashboard

import (
	"context"

	"github.com/anrted/opendeploy/internal/core/site"
	"github.com/anrted/opendeploy/internal/platform/events"
	"github.com/anrted/opendeploy/internal/platform/websocket"
)

func RegisterSiteLifecycleSubscriber(bus events.Bus, hub *websocket.Hub) events.UnsubscribeFn {
	eventTypes := []string{site.EventCreated, site.EventUpdated, site.EventDeleted, site.EventEnabled, site.EventDisabled}
	unsubscribers := make([]events.UnsubscribeFn, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		currentEventType := eventType
		unsubscribers = append(unsubscribers, events.SubscribePayload(bus, currentEventType, func(_ context.Context, data site.LifecycleEventData) error {
			hub.BroadcastToRoom("dashboard", websocket.Message{
				Type: "site_changed",
				Payload: map[string]any{
					"event": currentEventType,
					"site":  data,
				},
			})
			return nil
		}))
	}
	return func() {
		for _, unsubscribe := range unsubscribers {
			unsubscribe()
		}
	}
}
