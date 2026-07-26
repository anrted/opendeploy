package middleware

import (
	"net/http"
	"strings"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

// Auth validates the JWT Bearer token from the Authorization header.
// On success it stores the authenticated Principal in the request context.
// On failure it returns 401 Unauthorized.
func Auth(jwt *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				// Fallback to query parameter (useful for WebSockets)
				tokenStr = r.URL.Query().Get("token")
			}
			if tokenStr == "" {
				writeError(w, apperrors.Unauthorized("missing or malformed Authorization header or token query parameter"))
				return
			}

			claims, err := jwt.Validate(tokenStr)
			if err != nil {
				writeError(w, apperrors.New(http.StatusUnauthorized, apperrors.CodeTokenInvalid, "invalid or expired token"))
				return
			}

			principal := &auth.Principal{
				ID:       claims.UserID,
				Username: claims.Username,
				Role:     auth.Role(claims.Role),
			}
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission verifies that the authenticated principal holds the
// requested permission. Must be chained after Auth.
func RequirePermission(perm auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := auth.PrincipalFromContext(r.Context())
			if principal == nil {
				writeError(w, apperrors.Unauthorized("not authenticated"))
				return
			}
			if !principal.Role.HasPermission(perm) {
				writeError(w, apperrors.Forbidden("insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken parses "Bearer <token>" from the Authorization header.
func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
