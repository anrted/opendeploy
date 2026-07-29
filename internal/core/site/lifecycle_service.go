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
	if site.App.AppType != "" {
		if err := s.applyAppConfig(ctx, site.App.AppType, contract.SiteDelete, site); err != nil {
			s.logger.ErrorContext(ctx, "failed to remove app server config during site deletion", "error", err, "site_id", id)
		}
	}
	if err := s.applySiteConfig(ctx, site.ModuleID, contract.SiteDelete, site); err != nil {
		s.logger.ErrorContext(ctx, "failed to remove web server config during site deletion", "error", err, "site_id", id)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		// Rollback configuration if DB deletion fails
		_ = s.applySiteConfig(ctx, site.ModuleID, contract.SiteUpsert, site)
		if site.App.AppType != "" {
			_ = s.applyAppConfig(ctx, site.App.AppType, contract.SiteUpsert, site)
		}
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
	originalState := site.State
	site.State = state
	if err := s.repo.Update(ctx, site); err != nil {
		return err
	}

	rollback := func(err error, message string) error {
		s.logger.ErrorContext(ctx, message, "error", err, "site_id", id)
		// Revert DB state
		site.State = originalState
		_ = s.repo.Update(ctx, site)
		
		// Attempt to restore previous config state
		revertAction := contract.SiteDisable
		if originalState == StateActive {
			revertAction = contract.SiteEnable
		}
		_ = s.applySiteConfig(ctx, site.ModuleID, revertAction, site)
		if site.App.AppType != "" {
			_ = s.applyAppConfig(ctx, site.App.AppType, revertAction, site)
		}
		return err
	}

	if site.App.AppType != "" {
		if err := s.applyAppConfig(ctx, site.App.AppType, action, site); err != nil {
			message := "failed to enable app server config"
			if action == contract.SiteDisable {
				message = "failed to disable app server config"
			}
			return rollback(err, message)
		}
	}
	
	if err := s.applySiteConfig(ctx, site.ModuleID, action, site); err != nil {
		message := "failed to enable web server config"
		if action == contract.SiteDisable {
			message = "failed to disable web server config"
		}
		return rollback(err, message)
	}
	
	if action == contract.SiteEnable {
		if err := s.verifySiteHealth(ctx, site); err != nil {
			return rollback(err, "health check failed after enable")
		}
	}
	s.publishLifecycle(ctx, eventType, site, userID, ip)
	return nil
}
