package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
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
				"request_id", chiMiddleware.GetReqID(r.Context()),
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

// Unwrap allows net/http.ResponseController to reach the original writer.
func (lw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lw.ResponseWriter
}

// Hijack preserves WebSocket and other HTTP upgrade support through the
// logging wrapper.
func (lw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := lw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying response writer does not support hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err == nil {
		lw.statusCode = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

// Flush preserves streaming response support through the logging wrapper.
func (lw *loggingResponseWriter) Flush() {
	if flusher, ok := lw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
