package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger logs every HTTP request with method, path, status, and latency.
// It wraps the ResponseWriter to capture the status code written by the handler.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(lw, r)

			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", lw.statusCode,
				"latency_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}
