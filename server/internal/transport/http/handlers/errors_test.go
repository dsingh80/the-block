package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsingh80/the-block/server/internal/domain"
)

func TestWriteError_UnmappedDomainErrorCodeFallsBackTo500(t *testing.T) {
	// A *domain.DomainError whose Code has no entry in domainErrorResponses --
	// e.g. a future use-case introducing a new sentinel and forgetting to
	// register its envelope -- must still degrade to a generic 500 rather than
	// panic on the missing map entry or leak a zero-value status/message.
	err := &domain.DomainError{Code: "some_future_code_nobody_registered"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	writeError(rec, req, err)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for an unmapped DomainError code", rec.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", decodeErr, rec.Body.String())
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error.code = %q, want %q (the generic fallback, not the unmapped code echoed back)", body.Error.Code, "internal_error")
	}
}

func TestWriteError_NonDomainErrorReturns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	writeError(rec, req, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for a plain, non-domain error", rec.Code)
	}
}
