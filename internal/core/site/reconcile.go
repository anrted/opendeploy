package site

import (
	"context"
	"errors"
	"fmt"

	"github.com/anrted/opendeploy/pkg/contract"
)

func managesAppServer(appType string) bool {
	return appType != "" && appType != "static" && appType != "proxy"
}

func appConfigurationChanged(previous, current *Site) bool {
	if previous.App.AppType != current.App.AppType {
		return true
	}
	previousVersion := ""
	currentVersion := ""
	if previous.App.AppVersion != nil {
		previousVersion = *previous.App.AppVersion
	}
	if current.App.AppVersion != nil {
		currentVersion = *current.App.AppVersion
	}
	return previousVersion != currentVersion
}

// ReconcileActive repairs generated runtime configuration for sites that were
// created by an older OpenDeploy version.
func (s *Service) ReconcileActive(ctx context.Context) error {
	sites, err := s.repo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list sites for reconciliation: %w", err)
	}

	var reconciliationErrors []error
	for i := range sites {
		current := &sites[i]
		if current.State != StateActive {
			continue
		}
		if managesAppServer(current.App.AppType) {
			if err := s.applyAppConfig(ctx, current.App.AppType, contract.SiteUpsert, current); err != nil {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("site %s app config: %w", current.ID, err))
				continue
			}
		}
		if err := s.applySiteConfig(ctx, current.ModuleID, contract.SiteUpsert, current); err != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("site %s web config: %w", current.ID, err))
		}
	}
	return errors.Join(reconciliationErrors...)
}
