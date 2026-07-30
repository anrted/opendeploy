package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type countingLogHandler struct {
	count int
}

func (h *countingLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *countingLogHandler) Handle(context.Context, slog.Record) error {
	h.count++
	return nil
}
func (h *countingLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *countingLogHandler) WithGroup(string) slog.Handler      { return h }

func TestLoggerOmitsOnlyStatusOK(t *testing.T) {
	logHandler := &countingLogHandler{}
	handler := Logger(slog.New(logHandler))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	if logHandler.count != 0 {
		t.Fatalf("200 response produced %d log records", logHandler.count)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/changed", nil))
	if logHandler.count != 1 {
		t.Fatalf("204 response produced total %d log records, want 1", logHandler.count)
	}
}

func TestLoggerPreservesHijacker(t *testing.T) {
	handler := Logger(slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Hijacker); !ok {
			http.Error(w, "hijacker unavailable", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}
