package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recover wraps a handler so a panic returns 500 instead of crashing the
// process, and logs the panic + stack + request id before responding
// (guidelines/06-backend-architecture.md) -- one bad request must not take
// down the whole server. Writes its own minimal, fixed JSON body directly
// rather than importing the parent transport/http package's response helpers,
// which would create an import cycle (router.go imports this package to
// build the middleware chain); the body here never varies, so there's
// nothing worth sharing.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered [request_id=%s]: %v\n%s",
					RequestIDFromContext(r.Context()), rec, debug.Stack())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"Something went wrong."}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
