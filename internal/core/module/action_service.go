package module

import (
	"context"
	"fmt"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

// ExecuteAction is part of the stable Service facade; dynamic action dispatch
// is isolated from lifecycle and query operations in this file.
func (s *Service) ExecuteAction(ctx context.Context, id, actionID, userID, ip string) error {
	module := s.registry.Find(id)
	if module == nil {
		return apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	if err := module.ExecuteAction(ctx, actionID); err != nil {
		return apperrors.Internal(fmt.Sprintf("execute action %s on module %s", actionID, id), err)
	}
	s.recordAudit(ctx, userID, "module.action."+actionID, id, ip, audit.StatusSuccess)
	return nil
}
