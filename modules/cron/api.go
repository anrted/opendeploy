package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
)

func (m *Module) registerRoutes(router contract.Router) {
	router.Get("/jobs", m.httpList)
	router.Get("/jobs/{id}", m.httpGet)
	router.Post("/jobs", m.httpCreate)
	router.Put("/jobs/{id}", m.httpUpdate)
	router.Delete("/jobs/{id}", m.httpDelete)
	router.Post("/jobs/{id}/enable", m.httpEnable)
	router.Post("/jobs/{id}/disable", m.httpDisable)
	router.Post("/jobs/{id}/run", m.httpRun)
	router.Post("/jobs/{id}/duplicate", m.httpDuplicate)
	router.Get("/jobs/{id}/history", m.httpHistory)
	router.Get("/jobs/{id}/logs", m.httpHistory)
	router.Post("/validate", m.httpValidate)
	router.Get("/templates", m.httpTemplates)
	router.Get("/export", m.httpExport)
	router.Post("/import", m.httpImport)
}

func request(value interface{}) *http.Request      { return value.(*http.Request) }
func writer(value interface{}) http.ResponseWriter { return value.(http.ResponseWriter) }

func jobID(r *http.Request) string {
	path := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	for index := len(parts) - 1; index >= 0; index-- {
		if parts[index] == "jobs" && index+1 < len(parts) {
			return parts[index+1]
		}
	}
	return ""
}

func (m *Module) httpList(w, r interface{}) {
	jobs, err := m.agent.CronList(request(r).Context())
	writeJSON(writer(w), http.StatusOK, jobs, err)
}
func (m *Module) httpGet(w, r interface{}) {
	req := request(r)
	job, err := m.agent.CronGet(req.Context(), jobID(req))
	writeJSON(writer(w), http.StatusOK, job, err)
}
func (m *Module) httpCreate(w, r interface{}) {
	req := request(r)
	var job contract.CronJob
	if err := decodeJSON(req, &job); err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	created, err := m.agent.CronCreate(req.Context(), job)
	writeJSON(writer(w), http.StatusCreated, created, err)
}
func (m *Module) httpUpdate(w, r interface{}) {
	req := request(r)
	var job contract.CronJob
	if err := decodeJSON(req, &job); err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	job.ID = jobID(req)
	updated, err := m.agent.CronUpdate(req.Context(), job)
	writeJSON(writer(w), http.StatusOK, updated, err)
}
func (m *Module) httpDelete(w, r interface{}) {
	req := request(r)
	err := m.agent.CronDelete(req.Context(), jobID(req))
	writeJSON(writer(w), http.StatusOK, map[string]bool{"success": err == nil}, err)
}
func (m *Module) httpEnable(w, r interface{}) {
	req := request(r)
	job, err := m.agent.CronEnable(req.Context(), jobID(req))
	writeJSON(writer(w), http.StatusOK, job, err)
}
func (m *Module) httpDisable(w, r interface{}) {
	req := request(r)
	job, err := m.agent.CronDisable(req.Context(), jobID(req))
	writeJSON(writer(w), http.StatusOK, job, err)
}
func (m *Module) httpRun(w, r interface{}) {
	req := request(r)
	id := jobID(req)
	actor := req.Header.Get("X-OpenDeploy-Actor")
	if m.tasks == nil {
		run, err := m.agent.CronRun(req.Context(), id, "manual", actor)
		writeJSON(writer(w), http.StatusOK, run, err)
		return
	}
	job, err := m.agent.CronGet(req.Context(), id)
	if err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	taskID, err := m.tasks.StartTask(req.Context(), "Run Cron: "+job.Name, "cron_run", map[string]string{
		"module_id": "cron", "cron_id": id,
	}, func(ctx context.Context) (string, error) {
		run, runErr := m.agent.CronRun(ctx, id, "manual", actor)
		content, _ := json.Marshal(run)
		return string(content), runErr
	})
	writeJSON(writer(w), http.StatusAccepted, map[string]string{"job_id": taskID}, err)
}
func (m *Module) httpDuplicate(w, r interface{}) {
	req := request(r)
	job, err := m.agent.CronGet(req.Context(), jobID(req))
	if err == nil {
		job.ID, job.Name, job.Enabled = nextCopyID(job.ID), job.Name+" (copy)", false
		job, err = m.agent.CronCreate(req.Context(), *job)
	}
	writeJSON(writer(w), http.StatusCreated, job, err)
}
func (m *Module) httpHistory(w, r interface{}) {
	req := request(r)
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	runs, err := m.agent.CronHistory(req.Context(), jobID(req), limit)
	writeJSON(writer(w), http.StatusOK, runs, err)
}
func (m *Module) httpValidate(w, r interface{}) {
	req := request(r)
	var job contract.CronJob
	if err := decodeJSON(req, &job); err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	result, err := m.agent.CronValidate(req.Context(), job)
	writeJSON(writer(w), http.StatusOK, result, err)
}
func (m *Module) httpTemplates(w, _ interface{}) {
	writeJSON(writer(w), http.StatusOK, templates(), nil)
}
func (m *Module) httpExport(w, r interface{}) {
	req := request(r)
	format := strings.ToLower(req.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	jobs, err := m.agent.CronList(req.Context())
	if err != nil {
		writeJSON(writer(w), http.StatusInternalServerError, nil, err)
		return
	}
	content, contentType, err := exportJobs(jobs, format)
	if err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	writer(w).Header().Set("Content-Type", contentType)
	writer(w).Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="opendeploy-cron.%s"`, formatExtension(format)))
	writer(w).WriteHeader(http.StatusOK)
	_, _ = writer(w).Write(content)
}
func (m *Module) httpImport(w, r interface{}) {
	req := request(r)
	format := strings.ToLower(req.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	content, err := io.ReadAll(io.LimitReader(req.Body, 2<<20))
	if err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	jobs, err := importJobs(content, format)
	if err != nil {
		writeJSON(writer(w), http.StatusBadRequest, nil, err)
		return
	}
	results := make([]contract.CronJob, 0, len(jobs))
	for _, job := range jobs {
		result, createErr := m.agent.CronCreate(req.Context(), job)
		if createErr != nil {
			writeJSON(writer(w), http.StatusBadRequest, nil, createErr)
			return
		}
		results = append(results, *result)
	}
	writeJSON(writer(w), http.StatusCreated, results, nil)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status = http.StatusBadRequest
		data = map[string]any{"error": map[string]string{"code": "cron_error", "message": err.Error()}}
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func nextCopyID(id string) string { return id + "-copy" }
