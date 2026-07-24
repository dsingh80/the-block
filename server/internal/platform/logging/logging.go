// Package logging wires log/slog to stdout only (guidelines/06-backend-architecture.md,
// "Logging") -- no rotation library, no file path, no volume mount for logs at
// all. Docker's own json-file logging driver already writes every container's
// stdout to disk and rotates it -- more battle-tested than a third-party Go
// library, and it removes a dependency entirely.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// New builds the JSON logger every binary in this system uses. w is os.Stdout
// at every real call site; it's a parameter (not hardcoded) purely so this is
// unit-testable against a buffer. Deliberately never wraps w in a
// bufio.Writer: the point of "crash-safe" is that a crash gets no chance to
// flush a buffer it doesn't have -- each line is handed to Docker's logging
// driver as it's written.
func New(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

type contextKey int

const loggerKey contextKey = iota

// WithLogger attaches logger to ctx, retrievable via FromContext. This is how
// request_id (attached by middleware.RequestID) and, once resolved,
// session_id (attached by middleware.Session) thread through as structured
// fields on every log line from that point in the request downward
// (guidelines/06-backend-architecture.md) -- each middleware wraps whatever
// logger it finds in context with .With(...) and re-attaches the child, so
// fields accumulate rather than replace each other.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext returns the logger attached via WithLogger, or slog.Default()
// if none was attached -- every call site gets a usable logger, never nil.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
