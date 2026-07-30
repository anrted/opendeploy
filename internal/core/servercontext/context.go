// Package servercontext carries the selected infrastructure target through
// the HTTP request without coupling handlers to transport details.
package servercontext

import (
	"context"
	"net/http"
	"strings"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

const LocalID = "local"

type key struct{}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverID := strings.TrimSpace(r.Header.Get("X-Server-ID"))
		if serverID == "" {
			serverID = LocalID
		}
		ctx := context.WithValue(r.Context(), key{}, serverID)
		w.Header().Set("X-Server-ID", serverID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireMigratedCapability prevents a remote selection from ever leaking
// local data while capability adapters are being migrated.
func RequireMigratedCapability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsLocal(r.Context()) || remotePathAvailable(r) {
			next.ServeHTTP(w, r)
			return
		}
		apperrors.WriteHTTP(w, apperrors.New(
			http.StatusNotImplemented,
			apperrors.CodeCapabilityUnavailable,
			"this capability is not available through the remote control plane yet",
		))
	})
}

func remotePathAvailable(r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	if strings.HasPrefix(path, "/servers") || strings.HasPrefix(path, "/auth") {
		return true
	}
	if r.Method == http.MethodGet && path == "/dashboard" {
		return true
	}
	return strings.HasPrefix(path, "/system/processes") ||
		strings.HasPrefix(path, "/services") ||
		strings.HasPrefix(path, "/modules/firewall") ||
		strings.HasPrefix(path, "/modules/cron")
}

func ID(ctx context.Context) string {
	if value, ok := ctx.Value(key{}).(string); ok && value != "" {
		return value
	}
	return LocalID
}

func WithID(ctx context.Context, serverID string) context.Context {
	if strings.TrimSpace(serverID) == "" {
		serverID = LocalID
	}
	return context.WithValue(ctx, key{}, serverID)
}

func IsLocal(ctx context.Context) bool {
	return ID(ctx) == LocalID
}
