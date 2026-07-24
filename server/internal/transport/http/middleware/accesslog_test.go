package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// TestAccessLog_PreservesHijackForWebSocketUpgrades guards against a real
// gotcha: statusRecorder embeds http.ResponseWriter as an interface field,
// which only promotes that interface's own methods (Header/Write/WriteHeader) --
// NOT http.Hijacker's Hijack, even though the real *http.response underneath
// supports it. Without statusRecorder's own Hijack forwarding it,
// gorilla/websocket's Upgrade would fail its own http.Hijacker assertion for
// every WS request the moment it passed through this middleware
// (guidelines/06-backend-architecture.md). httptest.NewRecorder() can't catch
// this (it never implements Hijacker either way) -- this needs a real
// httptest.NewServer, whose ResponseWriter genuinely supports hijacking.
func TestAccessLog_PreservesHijackForWebSocketUpgrades(t *testing.T) {
	done := make(chan struct{})
	var hijackOK bool
	var hijackErr error

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		hj, ok := w.(http.Hijacker)
		hijackOK = ok
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		hijackErr = err
		if err == nil {
			conn.Close()
		}
	})

	server := httptest.NewServer(AccessLog(next))
	defer server.Close()

	go func() {
		resp, err := http.Get(server.URL)
		if err == nil {
			resp.Body.Close()
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never completed within the timeout")
	}

	if !hijackOK {
		t.Fatal("ResponseWriter passed through AccessLog does not implement http.Hijacker, want it forwarded")
	}
	if hijackErr != nil {
		t.Errorf("Hijack() = %v, want nil", hijackErr)
	}
}
