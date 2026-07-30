package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

func TestRateLimitUsesIndependentSessionBuckets(t *testing.T) {
	handler := RateLimit(2)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, token := range []string{"session-a", "session-b"} {
		for requestNumber := 0; requestNumber < 2; requestNumber++ {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
			request.RemoteAddr = "192.0.2.10:1234"
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("token %q request %d: status = %d", token, requestNumber, response.Code)
			}
		}
	}
}

func TestRateLimitReturnsTypedErrorAndRetryAfter(t *testing.T) {
	handler := RateLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	request.RemoteAddr = "192.0.2.20:1234"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}
	var payload struct {
		Error struct {
			Code apperrors.ErrorCode `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != apperrors.CodeRateLimited {
		t.Fatalf("code = %q", payload.Error.Code)
	}
}

func TestClientKeyDoesNotContainBearerSecret(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer highly-sensitive-token")
	key := clientKey(request, "192.0.2.30")
	if key == "" || key == "session:highly-sensitive-token" {
		t.Fatalf("unsafe client key %q", key)
	}
}
