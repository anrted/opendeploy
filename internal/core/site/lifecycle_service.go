package site

import (
	"context"

	"github.com/anrted/opendeploy/pkg/contract"
)

// Delete removes a site and delegates configuration cleanup.
func (s *Service) Delete(ctx context.Context, id string, userID, ip string) error {
	if err := s.backupCritical(ctx, "delete", id); err != nil {
		return err
	}
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
	if err := s.backupCritical(ctx, "enable", id); err != nil {
		return err
	}
	return s.setState(ctx, id, StateActive, contract.SiteEnable, EventEnabled, userID, ip)
}

func (s *Service) Disable(ctx context.Context, id string, userID, ip string) error {
	if err := s.backupCritical(ctx, "disable", id); err != nil {
		return err
	}
	return s.setState(ctx, id, StateDisabled, contract.SiteDisable, EventDisabled, userID, ip)
}

func (s *Service) setState(
	ctx context.Context,
	id string,
	state State,
	action contract.SiteAction,
	eventType string,
	userID string,
	ip string,
) error {
	site, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	site.State = state
	if err := s.repo.Update(ctx, site); err != nil {
		return err
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, action, site); err != nil {
		message := "failed to enable web server config"
		if action == contract.SiteDisable {
			message = "failed to disable web server config"
		}
		s.logger.ErrorContext(ctx, message, "error", err, "site_id", id)
	}
	s.publishLifecycle(ctx, eventType, site, userID, ip)
	return nil
}
