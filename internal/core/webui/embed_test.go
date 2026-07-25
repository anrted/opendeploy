package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndexAndSPAFallback(t *testing.T) {
	handler := Handler()
	for _, target := range []string{"/", "/sites/example"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", target, response.Code)
		}
		if !strings.Contains(response.Body.String(), `<div id="app"></div>`) {
			t.Fatalf("%s did not return the Vue entry document", target)
		}
	}
}

func TestHandlerDoesNotMaskUnknownAPIEndpoints(t *testing.T) {
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown API endpoint returned %d, want 404", response.Code)
	}
}
