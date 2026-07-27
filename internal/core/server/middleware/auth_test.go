package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anrted/opendeploy/internal/core/auth"
)

func TestAuthRejectsTokenInQueryString(t *testing.T) {
	manager, err := auth.NewJWTManager("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Generate("user-id", "viewer", string(auth.RoleViewer), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := Auth(manager)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil).WithContext(context.Background())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || called {
		t.Fatalf("query token was accepted: status=%d called=%t", response.Code, called)
	}
}

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
