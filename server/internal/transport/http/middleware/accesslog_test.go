package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withCapturedSlogDefault swaps slog's package-level default for the duration
// of fn, restoring the original afterward -- AccessLog reads its logger via
// logging.FromContext, which falls back to slog.Default() when nothing more
// specific was attached (the case in these tests, which call AccessLog
// directly without RequestID/Session wrapping it).
func withCapturedSlogDefault(t *testing.T, fn func(buf *bytes.Buffer)) {
	t.Helper()
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(orig)
	fn(&buf)
}

func TestAccessLog_RecordsMethodPathAndStatus(t *testing.T) {
	withCapturedSlogDefault(t, func(buf *bytes.Buffer) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/listings/abc/bids", nil)
		rec := httptest.NewRecorder()
		AccessLog(next).ServeHTTP(rec, req)

		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
			t.Fatalf("log output isn't valid JSON: %v (%s)", err, buf.String())
		}
		if line["method"] != "POST" || line["path"] != "/v1/listings/abc/bids" {
			t.Errorf("line = %v, want method=POST path=/v1/listings/abc/bids", line)
		}
		if status, ok := line["status"].(float64); !ok || int(status) != http.StatusCreated {
			t.Errorf("status = %v, want %d", line["status"], http.StatusCreated)
		}
	})
}

func TestAccessLog_DefaultsTo200WhenHandlerNeverCallsWriteHeader(t *testing.T) {
	withCapturedSlogDefault(t, func(buf *bytes.Buffer) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ok")) // implicit 200, never calls WriteHeader explicitly
		})

		req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
		rec := httptest.NewRecorder()
		AccessLog(next).ServeHTTP(rec, req)

		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
			t.Fatalf("log output isn't valid JSON: %v (%s)", err, buf.String())
		}
		if status, ok := line["status"].(float64); !ok || int(status) != http.StatusOK {
			t.Errorf("status = %v, want %d (implicit)", line["status"], http.StatusOK)
		}
	})
}

// TestStatusRecorderHijack_NotSupportedWhenUnderlyingWriterCant guards the
// other half of Hijack's branch: httptest.NewRecorder() never implements
// http.Hijacker, so wrapping one must forward that absence as
// http.ErrNotSupported rather than panicking on a failed type assertion.
func TestStatusRecorderHijack_NotSupportedWhenUnderlyingWriterCant(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}

	_, _, err := rec.Hijack()

	if !errors.Is(err, http.ErrNotSupported) {
		t.Errorf("Hijack() error = %v, want %v", err, http.ErrNotSupported)
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
