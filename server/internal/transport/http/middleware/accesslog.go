package middleware

import (
	"bufio"
	"net"
	"net/http"
	"time"

	"github.com/dsingh80/the-block/server/internal/platform/logging"
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

// AccessLog logs method, path, status, and duration as structured fields for
// every request (guidelines/06-backend-architecture.md, "Logging"). Reads the
// logger via logging.FromContext rather than logging.RequestIDFromContext
// plus a format string, so request_id (and session_id, for anything logged
// downstream of Session) ride along as real fields, not string interpolation.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logging.FromContext(r.Context()).Info("request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}
