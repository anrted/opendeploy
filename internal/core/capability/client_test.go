package capability

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/anrted/opendeploy/internal/core/controlplane"
	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

func TestOfflineRemoteAgentReturnsTypedServiceUnavailable(t *testing.T) {
	client := NewClient(nil, controlplane.NewManager())
	ctx := servercontext.WithID(context.Background(), "remote-1")

	_, err := client.ProcessList(ctx)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want AppError", err)
	}
	if appErr.HTTPStatus != http.StatusServiceUnavailable || appErr.Code != apperrors.CodeAgentUnavailable {
		t.Fatalf("status/code = %d/%s", appErr.HTTPStatus, appErr.Code)
	}
}

func TestRemoteDeadlineReturnsGatewayTimeout(t *testing.T) {
	err := remoteError(context.DeadlineExceeded)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want AppError", err)
	}
	if appErr.HTTPStatus != http.StatusGatewayTimeout || appErr.Code != apperrors.CodeAgentTimeout {
		t.Fatalf("status/code = %d/%s", appErr.HTTPStatus, appErr.Code)
	}
}
