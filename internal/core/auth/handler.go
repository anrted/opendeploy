package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/internal/platform/logger"
	"github.com/gorilla/csrf"
)

// contextKeyPrincipal is the context key for storing the authenticated user.
type contextKeyPrincipal struct{}

// Handler implements the HTTP endpoints for the auth domain.
type Handler struct {
	service *Service
}

// NewHandler constructs an auth Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ─── Request / Response types ──────────────────────────────────────────────

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ─── HTTP handlers ─────────────────────────────────────────────────────────

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	if req.Username == "" || req.Password == "" {
		respondError(w, apperrors.InvalidInput("username and password are required"))
		return
	}

	ip := realIP(r)
	ua := r.UserAgent()

	pair, err := h.service.Login(r.Context(), req.Username, req.Password, ip, ua)
	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, http.StatusOK, pair)
}

// Logout handles POST /api/v1/auth/logout (requires auth middleware)
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFromContext(r.Context())
	if principal == nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	if err := h.service.Logout(r.Context(), principal.ID); err != nil {
		respondError(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	if req.RefreshToken == "" {
		respondError(w, apperrors.InvalidInput("refresh_token is required"))
		return
	}

	pair, err := h.service.Refresh(r.Context(), req.RefreshToken, realIP(r), r.UserAgent())
	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, http.StatusOK, pair)
}

// Me handles GET /api/v1/auth/me (requires auth middleware)
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFromContext(r.Context())
	if principal == nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	user, err := h.service.GetUser(r.Context(), principal.ID)
	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, http.StatusOK, user)
}

// CSRFToken handles GET /api/v1/auth/csrf.
// It simply provides a way for the frontend to get the CSRF token before logging in.
func (h *Handler) CSRFToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-CSRF-Token", csrf.Token(r))
	w.WriteHeader(http.StatusNoContent)
}

// ─── Context helpers ───────────────────────────────────────────────────────

// Principal carries the authenticated user identity extracted from JWT claims.
type Principal struct {
	ID       string
	Username string
	Role     Role
}

// WithPrincipal stores the Principal in the request context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKeyPrincipal{}, p)
}

// PrincipalFromContext extracts the Principal from the context.
// Returns nil if no principal is present (unauthenticated request).
func PrincipalFromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(contextKeyPrincipal{}).(*Principal); ok {
		return p
	}
	return nil
}

// ─── HTTP helpers ──────────────────────────────────────────────────────────

// respond writes a JSON success response.
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails at this point we cannot send a different status.
		// Log it and move on.
		_ = err
	}
}

// respondError writes a JSON error response derived from an AppError.
func respondError(w http.ResponseWriter, err error) {
	ae := apperrors.AsAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    ae.Code,
			"message": ae.Message,
		},
	})
}

// realIP returns the client IP from common proxy headers, falling back to RemoteAddr.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For may contain a comma-separated list; use the first entry.
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	return r.RemoteAddr
}

// unusedImport suppresses unused import warning for the logger package while
// keeping the import for future request-scoped logging.
var _ = logger.FromContext
