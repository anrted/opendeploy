package servercontext

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareDefaultsToLocal(t *testing.T) {
	var got string
	handler := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ID(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil))
	if got != LocalID {
		t.Fatalf("got %q, want local", got)
	}
}

func TestUnmigratedRemoteCapabilityIsRejected(t *testing.T) {
	handler := Middleware(RequireMigratedCapability(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unmigrated handler was called")
	})))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sites", nil)
	request.Header.Set("X-Server-ID", "remote-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("got HTTP %d, want %d", response.Code, http.StatusNotImplemented)
	}
}

func TestMigratedRemoteCapabilityIsAllowed(t *testing.T) {
	called := false
	handler := Middleware(RequireMigratedCapability(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/processes", nil)
	request.Header.Set("X-Server-ID", "remote-1")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if !called {
		t.Fatal("migrated handler was not called")
	}
}
