package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLog_RecordsMethodPathAndStatus(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/abc/bids", nil)
	rec := httptest.NewRecorder()
	AccessLog(next).ServeHTTP(rec, req)

	line := buf.String()
	for _, want := range []string{"POST", "/v1/listings/abc/bids", "201"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q does not contain %q", line, want)
		}
	}
}

func TestAccessLog_DefaultsTo200WhenHandlerNeverCallsWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // implicit 200, never calls WriteHeader explicitly
	})

	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	rec := httptest.NewRecorder()
	AccessLog(next).ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), "200") {
		t.Errorf("log line %q does not contain the implicit 200 status", buf.String())
	}
}
