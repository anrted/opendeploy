package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

// RateLimit implements a simple per-IP sliding-window rate limiter.
// It does not require an external store (Redis etc.) and is suitable for MVP.
// For production consider a distributed limiter when multiple Core instances run.
func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := newIPLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				writeError(w, apperrors.New(
					http.StatusTooManyRequests,
					apperrors.CodeInvalidInput,
					"rate limit exceeded, please slow down",
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── ipLimiter ─────────────────────────────────────────────────────────────

type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func newIPLimiter(limit int) *ipLimiter {
	l := &ipLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
	}
	// Periodically clean up stale buckets.
	go l.cleanup()
	return l
}

// Allow returns true if the request from ip is within the rate limit.
func (l *ipLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok || now.After(b.windowEnd) {
		l.buckets[ip] = &bucket{count: 1, windowEnd: now.Add(time.Minute)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

// cleanup removes expired buckets every 2 minutes.
func (l *ipLimiter) cleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for ip, b := range l.buckets {
			if now.After(b.windowEnd) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// ─── Recover ───────────────────────────────────────────────────────────────

// Recover catches panics in HTTP handlers and returns a 500 response instead
// of crashing the server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				writeError(w, apperrors.Internal("an unexpected error occurred", nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── Shared helper ─────────────────────────────────────────────────────────

// writeError serialises an AppError to JSON and writes it to w.
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
