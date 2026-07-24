package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionInfo_ReturnsCreatedAtAndNeverTheToken(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	const secretToken = "super-secret-session-token"

	req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
	rec := httptest.NewRecorder()
	withFixedSessionAt(secretToken, createdAt, SessionInfo).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secretToken) {
		t.Errorf("response body contains the session token, want it never echoed back: %s", rec.Body.String())
	}

	var body struct {
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body.CreatedAt != "2026-01-01T12:00:00Z" {
		t.Errorf("created_at = %q, want %q", body.CreatedAt, "2026-01-01T12:00:00Z")
	}
}
