package site

import (
	"context"
	"fmt"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Configuration and lifecycle collaborators are isolated from CRUD
// orchestration while the Service API remains unchanged.
func (s *Service) applySiteConfig(ctx context.Context, moduleID string, action contract.SiteAction, site *Site) error {
	return s.deploy.Apply(ctx, moduleID, action, site)
}

func (s *Service) applyAppConfig(ctx context.Context, appType string, action contract.SiteAction, site *Site) error {
	return s.deploy.ApplyApp(ctx, appType, action, site)
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

func (s *Service) verifySiteHealth(ctx context.Context, site *Site) error {
	primaryDomain := site.Domains[0].Domain
	for _, d := range site.Domains {
		if d.Type == DomainPrimary {
			primaryDomain = d.Domain
			break
		}
	}

	// We use curl with Host header targeting localhost to check if the site responds.
	// Since we are running in an agent, we can execute this directly.
	if site.SSL != nil && site.SSL.ForceHTTPS {
		// If forced HTTPS, it should return a 301. We check if 301 is returned.
		// Alternatively, we can just allow 301, 200, 302, 403. Basically any valid HTTP response from Nginx.
	}

	if typed, ok := s.agent.(contract.SiteRuntimeAgentClient); ok && !servercontext.IsLocal(ctx) {
		var lastStatus int
		var lastErr error
		for i := 0; i < 10; i++ {
			lastStatus, lastErr = typed.SiteHTTPProbe(ctx, primaryDomain)
			if lastErr == nil && acceptedHealthStatus(lastStatus) {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
		return fmt.Errorf("health check failed for %s: received HTTP %d: %w", primaryDomain, lastStatus, lastErr)
	}

	var exitCode int
	var stdout, stderr string
	var err error

	// Retry up to 10 times (2 seconds) to allow Nginx to finish reloading
	for i := 0; i < 10; i++ {
		exitCode, stdout, stderr, err = s.agent.CommandExecute(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-H", fmt.Sprintf("Host: %s", primaryDomain), "http://127.0.0.1/")
		if err == nil && exitCode == 0 {
			// Accept 200 (OK), 301/302 (Redirects), 403 (Forbidden if no index), 404 (Not Found if no index)
			if stdout == "200" || stdout == "301" || stdout == "302" || stdout == "403" || stdout == "404" {
				return nil
			}
		}
		// wait 200ms before next retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	if err != nil {
		return fmt.Errorf("health check execution failed: %w", err)
	}
	return fmt.Errorf("health check failed for %s: received HTTP %s. Error: %s", primaryDomain, stdout, stderr)
}

func acceptedHealthStatus(status int) bool {
	return status == 200 || status == 301 || status == 302 || status == 403 || status == 404
}
