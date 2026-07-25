package module

import (
	"encoding/json"
	"net/http"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/go-chi/chi/v5"
)

// Handler exposes the module domain over HTTP.
type Handler struct {
	service *Service
}

// NewHandler creates a module Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/modules
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	views, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, views)
}

// Get handles GET /api/v1/modules/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	view, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, view)
}

// Install handles POST /api/v1/modules/{id}/install
func (h *Handler) Install(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	jobID, err := h.service.Install(r.Context(), id, userID, realIP(r))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// Uninstall handles POST /api/v1/modules/{id}/uninstall
func (h *Handler) Uninstall(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	jobID, err := h.service.Uninstall(r.Context(), id, userID, realIP(r))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// Enable handles POST /api/v1/modules/{id}/enable
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	if err := h.service.Enable(r.Context(), id, userID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "module enabled"})
}

// Disable handles POST /api/v1/modules/{id}/disable
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	if err := h.service.Disable(r.Context(), id, userID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "module disabled"})
}

// Restart handles POST /api/v1/modules/{id}/restart
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	if err := h.service.Restart(r.Context(), id, userID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "module restarted"})
}

// GetJob handles GET /api/v1/jobs/{id}
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := h.service.GetJob(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, job)
}

// ─── helpers ───────────────────────────────────────────────────────────────

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
		"error": map[string]any{
			"code":    ae.Code,
			"message": ae.Message,
		},
	})
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
