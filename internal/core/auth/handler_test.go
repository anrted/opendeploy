package auth

import (
	"net/http/httptest"
	"testing"
)

func TestRealIPIgnoresUntrustedProxyHeaders(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.5")
	request.Header.Set("X-Real-IP", "203.0.113.6")

	if got := realIP(request); got != "192.0.2.10" {
		t.Fatalf("realIP() = %q, want direct peer IP", got)
	}
}
