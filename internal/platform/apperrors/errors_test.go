package apperrors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteHTTPUsesStructuredSafeEnvelope(t *testing.T) {
	response := httptest.NewRecorder()
	WriteHTTP(response, Internal("unable to complete operation", errSensitive))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("X-Error-ID") == "" {
		t.Fatal("missing X-Error-ID correlation header")
	}
	var payload struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"code", "message", "details", "recommendation", "error_id"} {
		if payload.Error[key] == nil || payload.Error[key] == "" {
			t.Errorf("missing %s: %#v", key, payload.Error)
		}
	}
	if body := response.Body.String(); contains(body, "database-password") {
		t.Fatalf("internal cause leaked: %s", body)
	}
}

func TestInternalPreservesTypedOperationalError(t *testing.T) {
	original := AgentUnavailable(errSensitive)
	if got := Internal("service operation failed", original); got != original {
		t.Fatalf("typed error was replaced: %#v", got)
	}
}

var errSensitive = testError("database-password")

type testError string

func (err testError) Error() string { return string(err) }

func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
