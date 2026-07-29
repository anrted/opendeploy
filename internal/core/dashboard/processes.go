package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListProcesses handles GET /api/v1/system/processes
func (h *Handler) ListProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := h.service.agent.ProcessList(r.Context())
	if err != nil {
		writeError(w, apperrors.Internal("failed to list processes", err))
		return
	}
	respond(w, http.StatusOK, procs)
}

// KillProcess handles POST /api/v1/system/processes/{pid}/kill
func (h *Handler) KillProcess(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		writeError(w, apperrors.InvalidInput("invalid pid"))
		return
	}

	force := r.URL.Query().Get("force") == "true"

	if err := h.service.agent.ProcessKill(r.Context(), pid, force); err != nil {
		if errors.Is(err, ErrProtectedProcess) || status.Code(err) == codes.FailedPrecondition {
			writeError(w, apperrors.New(
				http.StatusConflict,
				apperrors.CodeConflict,
				"the selected process is protected; manage system services through Services",
			))
			return
		}
		writeError(w, apperrors.Internal("failed to kill process", err))
		return
	}
	respond(w, http.StatusOK, map[string]bool{"success": true})
}

// ErrProtectedProcess is retained for non-gRPC dashboard agents used in tests.
var ErrProtectedProcess = errors.New("protected process")
