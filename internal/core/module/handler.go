package module

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// Status handles GET /api/v1/modules/{id}/status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.service.Status(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, status)
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

// ExecuteAction handles POST /api/v1/modules/{id}/actions/{actionId}
func (h *Handler) ExecuteAction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	actionId := chi.URLParam(r, "actionId")
	principal := auth.PrincipalFromContext(r.Context())
	userID := ""
	if principal != nil {
		userID = principal.ID
	}

	if err := h.service.ExecuteAction(r.Context(), id, actionId, userID, realIP(r)); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "action executed successfully"})
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

func (h *Handler) HandleGetDataGridSchema(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	pageID := chi.URLParam(r, "pageId")
	schema, err := h.service.GetDataGridSchema(r.Context(), moduleID, pageID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

func (h *Handler) HandleGetDataGridData(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	pageID := chi.URLParam(r, "pageId")
	data, err := h.service.GetDataGridData(r.Context(), moduleID, pageID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) HandleExecuteDataGridAction(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	pageID := chi.URLParam(r, "pageId")
	actionID := chi.URLParam(r, "actionId")
	
	var payload map[string]any
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	
	err := h.service.ExecuteDataGridAction(r.Context(), moduleID, pageID, actionID, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	
	var settings map[string]any
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.service.SaveSettings(r.Context(), moduleID, settings); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleReadLog(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	logID := chi.URLParam(r, "logId")
	
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if parsed, err := strconv.Atoi(linesStr); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	
	output, err := h.service.ReadLog(r.Context(), moduleID, logID, lines)
	if err != nil {
		writeError(w, err)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}

func (h *Handler) HandleClearLog(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "id")
	logID := chi.URLParam(r, "logId")
	
	if err := h.service.ClearLog(r.Context(), moduleID, logID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
