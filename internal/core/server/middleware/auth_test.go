package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anrted/opendeploy/internal/core/auth"
)

func TestRequirePermissionRejectsViewerMutation(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequirePermission(auth.PermSiteDelete)(next)
	request := httptest.NewRequest(http.MethodDelete, "/sites/1", nil)
	request = request.WithContext(auth.WithPrincipal(request.Context(), &auth.Principal{Role: auth.RoleViewer}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestRequirePermissionAllowsAdminMutation(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequirePermission(auth.PermSiteDelete)(next)
	request := httptest.NewRequest(http.MethodDelete, "/sites/1", nil)
	request = request.WithContext(auth.WithPrincipal(request.Context(), &auth.Principal{Role: auth.RoleAdmin}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
}
