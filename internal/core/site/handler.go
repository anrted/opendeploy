package site

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

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

// ─── File Operations ───────────────────────────────────────────────────────

// ListFiles handles GET /api/v1/sites/{id}/files
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	files, err := h.service.ListFiles(r.Context(), id, path)
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, files)
}

// ReadFile handles GET /api/v1/sites/{id}/file
func (h *Handler) ReadFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, apperrors.InvalidInput("path is required"))
		return
	}
	content, err := h.service.ReadFile(r.Context(), id, path)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// WriteFile handles POST /api/v1/sites/{id}/file
// Accepts raw file content in body or multipart/form-data. For simplicity, raw body is accepted.
// Or we can expect JSON with "content" base64? Let's use multipart for file uploads.
func (h *Handler) WriteFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, apperrors.InvalidInput("path is required"))
		return
	}

	// Read from multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, apperrors.InvalidInput("failed to parse multipart form"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, apperrors.InvalidInput("file field is required"))
		return
	}
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	if err != nil {
		writeError(w, apperrors.InvalidInput("failed to read file content"))
		return
	}

	if err := h.service.WriteFile(r.Context(), id, path, content); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "file written successfully"})
}

// DeleteFile handles DELETE /api/v1/sites/{id}/file
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, apperrors.InvalidInput("path is required"))
		return
	}
	if err := h.service.DeleteFile(r.Context(), id, path); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "file deleted successfully"})
}

// CreateDirectory handles POST /api/v1/sites/{id}/directory
func (h *Handler) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// We read path from JSON body {"path": "/foo"} or query params
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.InvalidInput("invalid json body"))
		return
	}

	if req.Path == "" {
		writeError(w, apperrors.InvalidInput("path is required"))
		return
	}

	if err := h.service.CreateDirectory(r.Context(), id, req.Path); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]string{"message": "directory created successfully"})
}

// ─── helpers ───────────────────────────────────────────────────────────────

func principalOrEmpty(r *http.Request) *auth.Principal {
	if p := auth.PrincipalFromContext(r.Context()); p != nil {
		return p
	}
	return &auth.Principal{}
}

// BatchOperations handles POST /api/v1/sites/{id}/files/batch
func (h *Handler) BatchOperations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Action  string   `json:"action"` // "delete", "copy", "move", "chmod", "chown"
		Paths   []string `json:"paths"`
		DstPath string   `json:"dst_path"`
		Mode    uint32   `json:"mode"`
		Uid     int      `json:"uid"`
		Gid     int      `json:"gid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.InvalidInput("invalid json body"))
		return
	}
	if req.Action == "archive" {
		if err := h.service.CreateArchive(r.Context(), id, req.Paths, req.DstPath); err != nil {
			writeError(w, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"message": "archive created"})
		return
	}
	if req.Action == "extract" {
		if len(req.Paths) != 1 {
			writeError(w, apperrors.InvalidInput("extract requires exactly one archive"))
			return
		}
		if err := h.service.ExtractArchive(r.Context(), id, req.Paths[0], req.DstPath); err != nil {
			writeError(w, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"message": "archive extracted"})
		return
	}

	for _, p := range req.Paths {
		var err error
		switch req.Action {
		case "delete":
			err = h.service.DeleteFile(r.Context(), id, p)
		case "copy":
			dst := req.DstPath
			if strings.HasSuffix(dst, "/") {
				dst = dst + filepath.Base(p)
			}
			err = h.service.CopyFile(r.Context(), id, p, dst)
		case "move":
			dst := req.DstPath
			if strings.HasSuffix(dst, "/") {
				dst = dst + filepath.Base(p)
			}
			err = h.service.RenameFile(r.Context(), id, p, dst)
		case "chmod":
			err = h.service.ChmodFile(r.Context(), id, p, req.Mode)
		case "chown":
			err = h.service.ChownFile(r.Context(), id, p, req.Uid, req.Gid)
		default:
			err = apperrors.InvalidInput("unsupported action")
		}
		if err != nil {
			writeError(w, err)
			return
		}
	}
	respond(w, http.StatusOK, map[string]string{"message": "batch operation completed"})
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
	apperrors.WriteHTTP(w, err)
}
