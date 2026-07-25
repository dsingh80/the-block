package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/dsingh80/the-block/server/internal/platform/logging"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
)

// Recover wraps a handler so a panic returns 500 instead of crashing the
// process, and logs the panic + stack + request id before responding
// (guidelines/06-backend-architecture.md) -- one bad request must not take
// down the whole server. Uses transport/httputil rather than the parent
// transport/http package's own response helpers: router.go (in that parent
// package) imports this middleware package to build the chain, so depending
// back on it here would be an import cycle. httputil exists specifically as
// the dependency-light package both sides can import instead.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.FromContext(r.Context()).Error("panic recovered",
					"panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				httputil.WriteError(w, http.StatusInternalServerError, "internal_error",
					"Something went wrong.", nil, RequestIDFromContext(r.Context()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
