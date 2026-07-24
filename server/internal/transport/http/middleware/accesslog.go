package middleware

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder captures the status code a handler actually wrote --
// http.ResponseWriter doesn't expose that after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// AccessLog logs method, path, status, duration, and request id for every
// request. Uses the stdlib log package for now, same as elsewhere in
// server/ before structured logging is wired in (a later commit) --
// upgrading the destination later doesn't change any call site here
// (guidelines/06-backend-architecture.md).
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s [request_id=%s]",
			r.Method, r.URL.Path, rec.status, time.Since(start), RequestIDFromContext(r.Context()))
	})
}
