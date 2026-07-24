package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
)

// fakeBidStore is a spy for bidding.Store.
type fakeBidStore struct {
	result bidding.Result
	err    error

	lastListingID string
	lastSessionID string
	lastAmount    int64
	placeCalls    int
	buyNowCalls   int
}

func (f *fakeBidStore) PlaceBid(_ context.Context, listingID, sessionID string, amount int64) (bidding.Result, error) {
	f.placeCalls++
	f.lastListingID, f.lastSessionID, f.lastAmount = listingID, sessionID, amount
	if f.err != nil {
		return bidding.Result{}, f.err
	}
	return f.result, nil
}

func (f *fakeBidStore) BuyNow(_ context.Context, listingID, sessionID string) (bidding.Result, error) {
	f.buyNowCalls++
	f.lastListingID, f.lastSessionID = listingID, sessionID
	if f.err != nil {
		return bidding.Result{}, f.err
	}
	return f.result, nil
}

// fakeRateLimiter is a spy for bidding.RateLimiter. Zero value allows every
// request -- most tests here aren't about rate limiting, so that should never
// need to be set up explicitly.
type fakeRateLimiter struct {
	blocked bool
	err     error
	calls   int
}

func (f *fakeRateLimiter) Allow(_ context.Context, _, _ string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return !f.blocked, nil
}

func TestBidsPlace_Success(t *testing.T) {
	acceptedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeBidStore{result: bidding.Result{BidID: "bid-1", CurrentPrice: 21_500, BidCount: 3, AcceptedAt: acceptedAt}}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/bids", strings.NewReader(`{"amount": 21500}`))
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if store.lastListingID != sampleListingUUID || store.lastAmount != 21_500 {
		t.Errorf("PlaceBid called with (%q, _, %d), want (%q, _, 21500)", store.lastListingID, store.lastAmount, sampleListingUUID)
	}

	var resp dto.BidAcceptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	want := dto.BidAccept{BidID: "bid-1", CurrentBid: 21_500, BidCount: 3, AcceptedAt: "2026-01-01T12:00:00Z", Viewer: dto.AcceptedByCaller}
	if resp.Data != want {
		t.Errorf("data = %+v, want %+v", resp.Data, want)
	}
}

func TestBidsPlace_InvalidAmountReturns400WithoutCallingStore(t *testing.T) {
	tests := []struct{ name, body string }{
		{"missing amount", `{}`},
		{"zero amount", `{"amount": 0}`},
		{"negative amount", `{"amount": -100}`},
		{"malformed json", `not json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeBidStore{}
			h := NewBids(store, &fakeRateLimiter{})
			req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/bids", strings.NewReader(tt.body))
			req.SetPathValue("id", sampleListingUUID)
			rec := httptest.NewRecorder()
			h.Place(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 for body %q", rec.Code, tt.body)
			}
			if store.placeCalls != 0 {
				t.Errorf("PlaceBid called %d times, want 0 for invalid body %q", store.placeCalls, tt.body)
			}
		})
	}
}

func TestBidsPlace_MalformedIdReturns404WithoutCallingStore(t *testing.T) {
	store := &fakeBidStore{}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/not-a-uuid/bids", strings.NewReader(`{"amount": 21500}`))
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a malformed id", rec.Code)
	}
	if store.placeCalls != 0 {
		t.Errorf("PlaceBid called %d times, want 0", store.placeCalls)
	}
}

func TestBidsPlace_RateLimitedReturns429WithRetryAfterWithoutCallingStore(t *testing.T) {
	store := &fakeBidStore{}
	h := NewBids(store, &fakeRateLimiter{blocked: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/bids", strings.NewReader(`{"amount": 21500}`))
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body: %s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing on a 429")
	}
	if store.placeCalls != 0 {
		t.Errorf("PlaceBid called %d times, want 0 -- rate limiting must short-circuit before touching the store", store.placeCalls)
	}
}

func TestBidsPlace_DomainErrorsMapToDocumentedStatusCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"unknown listing", domain.ErrNotFound, http.StatusNotFound},
		{"auction not started", domain.ErrAuctionNotStarted, http.StatusConflict},
		{"auction already ended", domain.ErrAuctionEnded, http.StatusConflict},
		{"bid too low", domain.NewBidTooLowError(21_100), http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeBidStore{err: tt.err}
			h := NewBids(store, &fakeRateLimiter{})

			req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/bids", strings.NewReader(`{"amount": 21500}`))
			req.SetPathValue("id", sampleListingUUID)
			rec := httptest.NewRecorder()
			h.Place(rec, req)

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d for %v (body: %s)", rec.Code, tt.want, tt.err, rec.Body.String())
			}
		})
	}
}

func TestBidsPlace_BidTooLowIncludesMinimumInDetails(t *testing.T) {
	store := &fakeBidStore{err: domain.NewBidTooLowError(21_100)}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/bids", strings.NewReader(`{"amount": 21000}`))
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Error.Code != "bid_too_low" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "bid_too_low")
	}
	if got := body.Error.Details["minimum"]; got != float64(21_100) {
		t.Errorf("error.details.minimum = %v, want 21100", got)
	}
}

func TestBidsBuyNow_Success(t *testing.T) {
	acceptedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeBidStore{result: bidding.Result{BidID: "bid-2", CurrentPrice: 32_000, BidCount: 1, AcceptedAt: acceptedAt}}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/buy-now", nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.BuyNow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if store.buyNowCalls != 1 {
		t.Errorf("BuyNow called %d times, want 1", store.buyNowCalls)
	}

	var resp dto.BidAcceptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if resp.Data.Viewer != dto.AcceptedByCaller {
		t.Errorf("viewer = %+v, want %+v", resp.Data.Viewer, dto.AcceptedByCaller)
	}
}

func TestBidsBuyNow_BuyNowUnavailableReturns409(t *testing.T) {
	store := &fakeBidStore{err: domain.ErrBuyNowUnavailable}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/buy-now", nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.BuyNow(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestBidsBuyNow_MalformedIdReturns404WithoutCallingStore(t *testing.T) {
	store := &fakeBidStore{}
	h := NewBids(store, &fakeRateLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/not-a-uuid/buy-now", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.BuyNow(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a malformed id", rec.Code)
	}
	if store.buyNowCalls != 0 {
		t.Errorf("BuyNow called %d times, want 0", store.buyNowCalls)
	}
}

func TestBidsBuyNow_RateLimitedReturns429WithoutCallingStore(t *testing.T) {
	store := &fakeBidStore{}
	h := NewBids(store, &fakeRateLimiter{blocked: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+sampleListingUUID+"/buy-now", nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.BuyNow(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if store.buyNowCalls != 0 {
		t.Errorf("BuyNow called %d times, want 0", store.buyNowCalls)
	}
}
