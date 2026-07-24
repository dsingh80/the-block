// Package middleware holds the HTTP middleware chain (guidelines/06-backend-architecture.md).
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey int

const requestIDKey contextKey = iota

// RequestID assigns a fresh id to every request, or reuses an inbound
// X-Request-Id (so an upstream load balancer's own id threads through),
// makes it available via RequestIDFromContext, and echoes it back as a
// response header. The same id also lands in the structured log line and,
// for bidding endpoints, the bids.request_id column
// (guidelines/06-backend-architecture.md) -- one id across all three systems.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
