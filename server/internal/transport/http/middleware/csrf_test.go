package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRF_BlocksStateChangingRequestMissingTheHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/abc/bids", nil)
	rec := httptest.NewRecorder()
	CSRF(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Error("the wrapped handler was called despite the missing CSRF header")
	}
}

func TestCSRF_AllowsStateChangingRequestWithTheHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/abc/bids", nil)
	req.Header.Set(CSRFHeaderName, CSRFHeaderValue)
	rec := httptest.NewRecorder()
	CSRF(next).ServeHTTP(rec, req)

	if !called {
		t.Error("the wrapped handler was not called despite a valid CSRF header")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (default recorder status once the handler ran)", rec.Code, http.StatusOK)
	}
}

func TestCSRF_NeverBlocksReadOnlyRequests(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil) // no CSRF header at all
	rec := httptest.NewRecorder()
	CSRF(next).ServeHTTP(rec, req)

	if !called {
		t.Error("a GET request was blocked; CSRF should only apply to state-changing methods")
	}
}
