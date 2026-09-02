// Package logging is structured JSON logging with a trace id per request.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

type ctxKey string

const traceKey ctxKey = "trace_id"

func New(level string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func WithTrace(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceKey, traceID)
}

func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceKey).(string); ok {
		return v
	}
	return ""
}

// Str is the only way a user-supplied value should enter a log line. A newline
// in a name forges a second entry, and a log nobody can trust is worse than
// no log at all.
func Str(key, value string) slog.Attr {
	return slog.String(key, sanitize.LogValue(value))
}

// FromContext returns a logger already carrying the request's trace id.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if t := TraceID(ctx); t != "" {
		return base.With(slog.String("trace_id", t))
	}
	return base
}
