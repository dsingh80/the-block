package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsingh80/the-block/server/internal/platform/logging"
)

func TestRecover_PanicReturns500WithoutCrashingTheServer(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went badly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// The test function itself completing (rather than the panic propagating
	// out and failing the whole test binary) is half the proof here.
	Recover(panicking).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "internal_error")
	}
}

func TestRecover_PassesThroughANonPanickingHandlerUnchanged(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("fine"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Recover(ok).ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (Recover must not interfere with a normal response)", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "fine" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "fine")
	}
}

// TestRecover_LogsThePanicAsStructuredFields proves the panic log line carries
// the panic value and a stack trace as real structured fields (not baked into
// a formatted message string), and that it uses whatever per-request logger
// is already in context -- so request_id/session_id attached upstream ride
// along automatically (guidelines/06-backend-architecture.md, "Logging").
func TestRecover_LogsThePanicAsStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil)).With("request_id", "req-456")

	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went badly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(logging.WithLogger(req.Context(), testLogger))
	rec := httptest.NewRecorder()
	Recover(panicking).ServeHTTP(rec, req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log output isn't valid JSON: %v (%s)", err, buf.String())
	}
	if line["request_id"] != "req-456" {
		t.Errorf("request_id = %v, want %q (the upstream-attached logger)", line["request_id"], "req-456")
	}
	if line["panic"] != "something went badly wrong" {
		t.Errorf("panic = %v, want the panic value as its own field", line["panic"])
	}
	stack, _ := line["stack"].(string)
	if !strings.Contains(stack, "goroutine") {
		t.Errorf("stack field = %q, want it to look like a real stack trace", stack)
	}
}
