package middleware

import (
	"bufio"
	"log"
	"net"
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

// Hijack forwards to the underlying ResponseWriter's own Hijack. Without this,
// embedding http.ResponseWriter as an interface field only promotes that
// interface's own methods (Header/Write/WriteHeader) -- NOT http.Hijacker's
// Hijack, even though the concrete *http.response underneath supports it.
// gorilla/websocket's Upgrade asserts for http.Hijacker on whatever
// ResponseWriter it's handed; without this override, every WS upgrade request
// would fail the moment it passed through this middleware
// (guidelines/06-backend-architecture.md, "Caught by ... not by inspection" gotchas).
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hj.Hijack()
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
