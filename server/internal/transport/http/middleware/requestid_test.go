package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
