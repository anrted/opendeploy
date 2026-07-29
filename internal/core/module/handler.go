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

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	page, err := h.service.ListJobs(r.Context(), JobFilter{
		Query: r.URL.Query().Get("q"), State: JobState(r.URL.Query().Get("state")),
		Type: JobType(r.URL.Query().Get("type")), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, page)
}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	if err := h.service.CancelJob(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "task canceled"})
}

func (h *Handler) RetryJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := h.service.RetryJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteJob(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ───────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	apperrors.WriteHTTP(w, err)
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
			writeError(w, apperrors.InvalidInput("malformed JSON body"))
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
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}

	if err := h.service.SaveSettings(r.Context(), moduleID, settings); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ProtectionPresets(w http.ResponseWriter, r *http.Request) {
	presets, err := h.service.ProtectionPresets(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, presets)
}

func (h *Handler) PreviewProtectionPreset(w http.ResponseWriter, r *http.Request) {
	settings := map[string]any{}
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			writeError(w, apperrors.InvalidInput("malformed JSON body"))
			return
		}
	}
	preview, err := h.service.PreviewProtectionPreset(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "presetId"), settings)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, preview)
}

func presetActor(r *http.Request) (string, string) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		return "", realIP(r)
	}
	return principal.ID, realIP(r)
}

func (h *Handler) SaveProtectionPreset(w http.ResponseWriter, r *http.Request) {
	settings := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	userID, ip := presetActor(r)
	if err := h.service.SaveProtectionPreset(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "presetId"), settings, userID, ip); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "protection preset saved"})
}

func (h *Handler) ResetProtectionPreset(w http.ResponseWriter, r *http.Request) {
	userID, ip := presetActor(r)
	if err := h.service.ResetProtectionPreset(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "presetId"), userID, ip); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "protection preset reset"})
}

func (h *Handler) ToggleProtectionPreset(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, apperrors.InvalidInput("malformed JSON body"))
		return
	}
	userID, ip := presetActor(r)
	if err := h.service.SetProtectionPresetEnabled(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "presetId"), payload.Enabled, userID, ip); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "protection preset state updated"})
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
	if lines > 5000 {
		lines = 5000
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
