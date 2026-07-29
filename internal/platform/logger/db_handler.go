package logger

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// DBHandler wraps another slog.Handler and writes structured logs to SQLite.
type DBHandler struct {
	base     slog.Handler
	db       *sql.DB
	ch       chan slog.Record
	mu       sync.RWMutex
	done     chan struct{}
}

// NewDBHandler creates a new DBHandler wrapping the provided base handler.
// It starts a background worker to persist logs. Call Close() to drain.
func NewDBHandler(base slog.Handler) *DBHandler {
	h := &DBHandler{
		base: base,
		ch:   make(chan slog.Record, 10000), // Buffer to avoid blocking callers
		done: make(chan struct{}),
	}
	return h
}

// SetDB injects the database connection and starts the flush worker.
// This allows logger initialization before the DB is ready.
func (h *DBHandler) SetDB(db *sql.DB) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.db = db
	go h.worker()
}

// Enabled implements slog.Handler.
func (h *DBHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

// Handle implements slog.Handler. It passes the record to the base handler
// and queues it for database insertion.
func (h *DBHandler) Handle(ctx context.Context, r slog.Record) error {
	// Let base handler process it first
	err := h.base.Handle(ctx, r)
	
	// Queue for DB if it's not full, drop if full to prevent cascading failures
	select {
	case h.ch <- r:
	default:
	}

	return err
}

// WithAttrs implements slog.Handler.
func (h *DBHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DBHandler{
		base: h.base.WithAttrs(attrs),
		db:   h.db,
		ch:   h.ch,
		done: h.done,
	}
}

// WithGroup implements slog.Handler.
func (h *DBHandler) WithGroup(name string) slog.Handler {
	return &DBHandler{
		base: h.base.WithGroup(name),
		db:   h.db,
		ch:   h.ch,
		done: h.done,
	}
}

// Close stops the background worker and drains the channel.
func (h *DBHandler) Close() {
	close(h.ch)
	<-h.done
}

func (h *DBHandler) worker() {
	defer close(h.done)

	// Batch insertion for high throughput
	var batch []slog.Record
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case r, ok := <-h.ch:
			if !ok {
				h.flush(batch)
				return
			}
			batch = append(batch, r)
			if len(batch) >= 100 {
				h.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				h.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (h *DBHandler) flush(records []slog.Record) {
	h.mu.RLock()
	db := h.db
	h.mu.RUnlock()

	if db == nil || len(records) == 0 {
		return
	}

	tx, err := db.Begin()
	if err != nil {
		return // Failed to start transaction
	}
	defer tx.Rollback() // Rollback if not committed

	stmt, err := tx.Prepare(`
		INSERT INTO system_logs (
			timestamp, level, component, module, error_id, request_id, 
			user_id, duration_ms, endpoint, method, ip, message, stack_trace, attributes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, r := range records {
		attrs := make(map[string]any)
		var component, module, errorID, requestID, userID, endpoint, method, ip, stackTrace string
		var durationMs int64

		r.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "component":
				component = a.Value.String()
			case "module":
				module = a.Value.String()
			case "error_id":
				errorID = a.Value.String()
			case "request_id":
				requestID = a.Value.String()
			case "user", "user_id":
				userID = a.Value.String()
			case "latency_ms", "duration_ms":
				durationMs = a.Value.Int64()
			case "path", "endpoint":
				endpoint = a.Value.String()
			case "method":
				method = a.Value.String()
			case "remote", "ip":
				ip = a.Value.String()
			case "stack", "stack_trace":
				stackTrace = a.Value.String()
			default:
				attrs[a.Key] = a.Value.Any()
			}
			return true
		})

		attrBytes, _ := json.Marshal(attrs)

		_, _ = stmt.Exec(
			r.Time, r.Level.String(), component, module, errorID, requestID,
			userID, durationMs, endpoint, method, ip, r.Message, stackTrace, string(attrBytes),
		)
	}

	_ = tx.Commit()
}
