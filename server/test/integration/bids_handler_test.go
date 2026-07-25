//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/transport/http/handlers"
)

// TestBidsPlace_IdenticalRetryReturnsByteIdenticalResponse is the full-stack
// version of the claim proven at the BidStore level in bidding_test.go: not
// just that Redis replays the same Result, but that the HTTP response an
// actual retried request gets back is byte-identical to the original
// (guidelines/06-backend-architecture.md, "Idempotency") -- real Redis, the
// real handler, nothing faked.
func TestBidsPlace_IdenticalRetryReturnsByteIdenticalResponse(t *testing.T) {
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	rateLimiter := redisstore.NewRateLimiter(rdb, 100, time.Minute) // generous -- not what this test is about
	h := handlers.NewBids(store, rateLimiter)

	listingID := "b1111111-1111-1111-1111-111111111111"
	now := time.Now()
	primeListingState(t, rdb, listingID, 1_000, 0, now.Add(-time.Hour), now.Add(time.Hour))

	place := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+listingID+"/bids", strings.NewReader(`{"amount": 1100}`))
		req.SetPathValue("id", listingID)
		rec := httptest.NewRecorder()
		h.Place(rec, req)
		return rec
	}

	first := place()
	if first.Code != http.StatusCreated {
		t.Fatalf("first request status = %d, want 201 (body: %s)", first.Code, first.Body.String())
	}

	second := place()
	if second.Code != first.Code {
		t.Fatalf("retried status = %d, want %d (identical to the original)", second.Code, first.Code)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("retried body = %s, want byte-identical to the original %s", second.Body.String(), first.Body.String())
	}
}

// TestBidsBuyNow_IdenticalRetryReturnsByteIdenticalResponse mirrors the above
// for buy-now's (session, listing)-only idempotency key.
func TestBidsBuyNow_IdenticalRetryReturnsByteIdenticalResponse(t *testing.T) {
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	rateLimiter := redisstore.NewRateLimiter(rdb, 100, time.Minute)
	h := handlers.NewBids(store, rateLimiter)

	listingID := "b2222222-2222-2222-2222-222222222222"
	now := time.Now()
	err := rdb.HSet(context.Background(), redisstore.ListingStateKey(listingID),
		"current_price", 1_000, "bid_count", 0,
		"auction_start_ms", now.Add(-time.Hour).UnixMilli(),
		"auction_end_ms", now.Add(time.Hour).UnixMilli(),
		"version", 0, "high_bidder_session", "", "buy_now_price", 5_000,
	).Err()
	if err != nil {
		t.Fatalf("prime listing state: %v", err)
	}

	buyNow := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/listings/"+listingID+"/buy-now", nil)
		req.SetPathValue("id", listingID)
		rec := httptest.NewRecorder()
		h.BuyNow(rec, req)
		return rec
	}

	first := buyNow()
	if first.Code != http.StatusCreated {
		t.Fatalf("first request status = %d, want 201 (body: %s)", first.Code, first.Body.String())
	}

	second := buyNow()
	if second.Code != first.Code || second.Body.String() != first.Body.String() {
		t.Errorf("retried buy-now = {status:%d body:%s}, want byte-identical to the original {status:%d body:%s}",
			second.Code, second.Body.String(), first.Code, first.Body.String())
	}
}
