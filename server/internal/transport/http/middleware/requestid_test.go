package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsingh80/the-block/server/internal/platform/logging"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var seenInContext string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenInContext = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	RequestID(next).ServeHTTP(rec, req)

	if seenInContext == "" {
		t.Error("expected a generated request id to be readable from the handler's context")
	}
	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Error("expected X-Request-Id to be set on the response")
	} else if got != seenInContext {
		t.Errorf("response header X-Request-Id = %q, want it to match the context value %q", got, seenInContext)
	}
}

func TestRequestID_ReusesInboundHeader(t *testing.T) {
	var seenInContext string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenInContext = RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "upstream-provided-id")
	rec := httptest.NewRecorder()
	RequestID(next).ServeHTTP(rec, req)

	if seenInContext != "upstream-provided-id" {
		t.Errorf("context request id = %q, want the inbound header value preserved", seenInContext)
	}
	if got := rec.Header().Get("X-Request-Id"); got != "upstream-provided-id" {
		t.Errorf("response X-Request-Id = %q, want the inbound value echoed back unchanged", got)
	}
}

// TestRequestID_AttachesRequestIDToTheContextLogger proves request_id doesn't
// just live in context as a raw string (via RequestIDFromContext) but also
// rides along on the per-request logger every downstream log line uses
// (guidelines/06-backend-architecture.md, "Logging").
func TestRequestID_AttachesRequestIDToTheContextLogger(t *testing.T) {
	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.FromContext(r.Context()).Info("handler ran")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req-123")
	req = req.WithContext(logging.WithLogger(req.Context(), testLogger))
	rec := httptest.NewRecorder()
	RequestID(next).ServeHTTP(rec, req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log output isn't valid JSON: %v (%s)", err, buf.String())
	}
	if line["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want %q", line["request_id"], "req-123")
	}
}
