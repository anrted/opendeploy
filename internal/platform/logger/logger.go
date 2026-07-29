// Package logger provides structured, context-aware logging for OpenDeploy.
//
// It wraps the standard library's log/slog package, adding context propagation
// and caller information. All components should use this package instead of
// importing log/slog directly.
package logger

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// loggerKey is the context key used to store the logger.
var loggerKey = contextKey{}

// New creates a new structured logger with the specified level and format.
// format must be "json" or "text". Any other value defaults to "json".
// If filePath is non-empty, output is directed to that file in addition to stdout.
func New(level, format, filePath string) (*slog.Logger, error) {
	var w io.Writer = os.Stdout

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, err
		}
		w = io.MultiWriter(os.Stdout, f)
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	dbHandler := NewDBHandler(handler)
	globalDBHandler = dbHandler

	return slog.New(dbHandler), nil
}

var globalDBHandler *DBHandler

// SetDB injects the database connection into the global DB logger.
func SetDB(db *sql.DB) {
	if globalDBHandler != nil {
		globalDBHandler.SetDB(db)
	}
}

// WithContext stores the logger in the context and returns the derived context.
// Use this to propagate a logger through request-scoped goroutines.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves the logger stored in the context.
// If no logger is found, it returns the default slog logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// With returns a logger derived from the context logger with additional
// structured attributes. This is a convenience wrapper for request handlers.
func With(ctx context.Context, args ...any) *slog.Logger {
	return FromContext(ctx).With(args...)
}
