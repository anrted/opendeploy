package site

import (
	"time"

	"github.com/anrted/opendeploy/internal/platform/events"
)

const (
	EventCreated  = "site.created"
	EventUpdated  = "site.updated"
	EventDeleted  = "site.deleted"
	EventEnabled  = "site.enabled"
	EventDisabled = "site.disabled"
)

type LifecycleEventData struct {
	SiteID        string    `json:"site_id"`
	Name          string    `json:"name"`
	PrimaryDomain string    `json:"primary_domain"`
	ModuleID      string    `json:"module_id"`
	RootPath      string    `json:"root_path"`
	State         State     `json:"state"`
	AppType       string    `json:"app_type"`
	AppVersion    string    `json:"app_version,omitempty"`
	ActorID       string    `json:"actor_id,omitempty"`
	IPAddress     string    `json:"ip_address,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func newLifecycleEvent(eventType string, current *Site, actorID, ipAddress string) events.BaseEvent {
	primaryDomain := ""
	for _, domain := range current.Domains {
		if domain.Type == DomainPrimary {
			primaryDomain = domain.Domain
			break
		}
	}
	appVersion := ""
	if current.App.AppVersion != nil {
		appVersion = *current.App.AppVersion
	}
	data := LifecycleEventData{
		SiteID: current.ID, Name: current.Name, PrimaryDomain: primaryDomain,
		ModuleID: current.ModuleID, RootPath: current.RootPath, State: current.State,
		AppType: current.App.AppType, AppVersion: appVersion,
		ActorID: actorID, IPAddress: ipAddress, OccurredAt: time.Now().UTC(),
	}
	return events.NewBaseEvent(eventType, data)
}
