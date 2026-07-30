package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/go-chi/chi/v5/middleware"
)

// RateLimit applies a continuously refilling token bucket. Authenticated SPA
// sessions receive independent buckets, while a higher per-IP ceiling still
// prevents clients from evading the limiter with arbitrary bearer values.
// It does not require an external store (Redis etc.) and is suitable for MVP.
// For production consider a distributed limiter when multiple Core instances run.
func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	if requestsPerMinute < 1 {
		requestsPerMinute = 1
	}
	clientLimiter := newClientLimiter(requestsPerMinute)
	ipLimiter := newClientLimiter(requestsPerMinute * 10)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := clientKey(r, ip)
			ipAllowed, ipRetry := ipLimiter.Allow(ip)
			clientAllowed, clientRetry := clientLimiter.Allow(key)
			if !ipAllowed || !clientAllowed {
				retryAfter := maxDuration(ipRetry, clientRetry)
				w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
				writeError(w, apperrors.New(
					http.StatusTooManyRequests,
					apperrors.CodeRateLimited,
					"rate limit exceeded, please slow down",
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── ipLimiter ─────────────────────────────────────────────────────────────

type clientLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64
	rate     float64
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func newClientLimiter(limit int) *clientLimiter {
	l := &clientLimiter{
		buckets:  make(map[string]*bucket),
		capacity: float64(limit),
		rate:     float64(limit) / float64(time.Minute),
	}
	// Periodically clean up stale buckets.
	go l.cleanup()
	return l
}

// Allow consumes one token and returns the delay until another token is
// available when the bucket is empty.
func (l *clientLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.capacity - 1, lastSeen: now}
		return true, 0
	}
	elapsed := now.Sub(b.lastSeen)
	b.tokens = min(l.capacity, b.tokens+float64(elapsed)*l.rate)
	b.lastSeen = now
	if b.tokens < 1 {
		return false, time.Duration(math.Ceil((1 - b.tokens) / l.rate))
	}
	b.tokens--
	return true, 0
}

// cleanup removes expired buckets every 2 minutes.
func (l *clientLimiter) cleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for key, b := range l.buckets {
			if now.Sub(b.lastSeen) > 2*time.Minute {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}

func clientKey(r *http.Request, ip string) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authorization) > len("Bearer ") && strings.EqualFold(authorization[:len("Bearer ")], "Bearer ") {
		sum := sha256.Sum256([]byte(strings.TrimSpace(authorization[len("Bearer "):])))
		return "session:" + hex.EncodeToString(sum[:])
	}
	return "ip:" + ip
}

func retryAfterSeconds(delay time.Duration) string {
	seconds := int(math.Ceil(delay.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprint(seconds)
}

func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
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
				slog.ErrorContext(r.Context(), "panic recovered in HTTP handler",
					"panic", rv,
					"request_id", middleware.GetReqID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				writeError(w, apperrors.Internal("an unexpected error occurred", nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── Shared helper ─────────────────────────────────────────────────────────

// writeError serialises an AppError to JSON and writes it to w.
func writeError(w http.ResponseWriter, err error) {
	apperrors.WriteHTTP(w, err)
}
