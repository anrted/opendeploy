package servercontext

import (
	"encoding/json"
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
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "CAPABILITY_UNAVAILABLE" {
		t.Fatalf("code = %q", payload.Error.Code)
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

func TestRemoteCapabilityBoundary(t *testing.T) {
	allowed := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/dashboard"},
		{http.MethodPost, "/api/v1/system/processes/42/kill"},
		{http.MethodGet, "/api/v1/services"},
		{http.MethodGet, "/api/v1/modules/firewall/status"},
		{http.MethodGet, "/api/v1/modules/cron/jobs"},
		{http.MethodGet, "/api/v1/servers"},
	}
	for _, test := range allowed {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			called := false
			handler := Middleware(RequireMigratedCapability(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			})))
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("X-Server-ID", "remote-1")
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if !called {
				t.Fatal("migrated route was rejected")
			}
		})
	}

	rejected := []string{
		"/api/v1/sites",
		"/api/v1/modules",
		"/api/v1/settings",
		"/api/v1/updates",
		"/api/v1/tasks",
		"/api/v1/logs",
	}
	for _, path := range rejected {
		t.Run("reject "+path, func(t *testing.T) {
			handler := Middleware(RequireMigratedCapability(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("unmigrated route was called")
			})))
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("X-Server-ID", "remote-1")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotImplemented)
			}
		})
	}
}
