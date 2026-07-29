package logs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.SearchLogs)
	r.Get("/{id}", h.GetLog)
	return r
}

func (h *Handler) SearchLogs(w http.ResponseWriter, r *http.Request) {
	filter := LogFilter{
		Level:     r.URL.Query().Get("level"),
		Module:    r.URL.Query().Get("module"),
		Component: r.URL.Query().Get("component"),
		ErrorID:   r.URL.Query().Get("error_id"),
		RequestID: r.URL.Query().Get("request_id"),
		UserID:    r.URL.Query().Get("user_id"),
		Query:     r.URL.Query().Get("query"),
	}

	if s := r.URL.Query().Get("start_date"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.StartDate = t
		}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.EndDate = t
		}
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 100
	}

	res, err := h.service.Search(r.Context(), filter)
	if err != nil {
		apperrors.WriteHTTP(w, apperrors.Internal("failed to search logs", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (h *Handler) GetLog(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		apperrors.WriteHTTP(w, apperrors.InvalidInput("invalid log id"))
		return
	}

	l, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		apperrors.WriteHTTP(w, apperrors.Internal("failed to fetch log", err))
		return
	}
	if l == nil {
		apperrors.WriteHTTP(w, apperrors.NotFound("log entry not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l)
}
