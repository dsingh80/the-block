package httputil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	WriteJSON(w, 201, map[string]string{"hello": "world"})

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body did not decode as JSON: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("body = %+v, want {hello: world}", body)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, 409, "bid_too_low", "Enter at least $21,100.", map[string]any{"minimum": 21100}, "req-123")

	if w.Code != 409 {
		t.Errorf("status = %d, want 409", w.Code)
	}

	var decoded struct {
		Error struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			Details   map[string]any `json:"details"`
			RequestID string         `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("response body did not decode as the error envelope: %v", err)
	}
	if decoded.Error.Code != "bid_too_low" {
		t.Errorf("code = %q, want bid_too_low", decoded.Error.Code)
	}
	if decoded.Error.Message != "Enter at least $21,100." {
		t.Errorf("message = %q, want %q", decoded.Error.Message, "Enter at least $21,100.")
	}
	if decoded.Error.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", decoded.Error.RequestID)
	}
	if got, ok := decoded.Error.Details["minimum"]; !ok || got != float64(21100) {
		t.Errorf("details.minimum = %v, want 21100", got)
	}
}

func TestWriteError_NilDetailsOmitsTheField(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, 404, "not_found", "Listing not found.", nil, "req-456")

	var decoded map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("response body did not decode as JSON: %v", err)
	}
	errBody, _ := decoded["error"].(map[string]any)
	if _, present := errBody["details"]; present {
		t.Errorf("error envelope = %+v, want no details key when details is nil", errBody)
	}
}
