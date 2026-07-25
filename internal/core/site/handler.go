package site

import (
	"encoding/json"
	"net/http"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/go-chi/chi/v5"
)

// Handler exposes site management over HTTP.
type Handler struct {
	service *Service
}

// NewHandler constructs a site Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/sites
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sites, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, sites)
}

// Get handles GET /api/v1/sites/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	site, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, site)
}

// Create handles POST /api/v1/sites
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}

	principal := principalOrEmpty(r)
	site, err := h.service.Create(r.Context(), req, principal.ID, realIP(r))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusCreated, site)
}

// Update handles PUT /api/v1/sites/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}

	principal := principalOrEmpty(r)
	site, err := h.service.Update(r.Context(), id, req, principal.ID, realIP(r))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, site)
}

// Delete handles DELETE /api/v1/sites/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := principalOrEmpty(r)
	if err := h.service.Delete(r.Context(), id, principal.ID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "site deleted"})
}

// Enable handles POST /api/v1/sites/{id}/enable
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := principalOrEmpty(r)
	if err := h.service.Enable(r.Context(), id, principal.ID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "site enabled"})
}

// Disable handles POST /api/v1/sites/{id}/disable
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := principalOrEmpty(r)
	if err := h.service.Disable(r.Context(), id, principal.ID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "site disabled"})
}

// ─── helpers ───────────────────────────────────────────────────────────────

func principalOrEmpty(r *http.Request) *auth.Principal {
	if p := auth.PrincipalFromContext(r.Context()); p != nil {
		return p
	}
	return &auth.Principal{}
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	ae := apperrors.AsAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": ae.Code, "message": ae.Message},
	})
}
