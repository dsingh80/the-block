package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dsingh80/the-block/server/internal/platform/logging"
	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
)

// bidRateLimitBucket namespaces the bid/buy-now rate limit from any other
// bucket that might share a session's rate-limit key space later.
const bidRateLimitBucket = "bid"

// bidRateLimitRetryAfterSeconds is a fixed, generous hint -- the actual window
// is enforced server-side by RateLimiter regardless of what a client does with
// this header (guidelines/06-backend-architecture.md, "SOC2 principles mapping").
const bidRateLimitRetryAfterSeconds = "60"

// Bids serves the bid-accept endpoints, wiring bidding.Store's Lua-scripted
// atomic accept path to HTTP (guidelines/06-backend-architecture.md, "API design").
type Bids struct {
	store       bidding.Store
	rateLimiter bidding.RateLimiter
}

func NewBids(store bidding.Store, rateLimiter bidding.RateLimiter) *Bids {
	return &Bids{store: store, rateLimiter: rateLimiter}
}

type placeBidRequest struct {
	Amount int64 `json:"amount"`
}

// Place handles POST /v1/listings/{id}/bids, body {"amount": 21500}. No
// client-supplied idempotency key -- Store derives one from (session, listing,
// amount) itself (guidelines/06-backend-architecture.md, "Idempotency").
func (h *Bids) Place(w http.ResponseWriter, r *http.Request) {
	id, ok := parseListingID(w, r)
	if !ok {
		return
	}
	sessionToken := sessionTokenOf(r)

	if !h.checkRateLimit(w, r, sessionToken) {
		return
	}

	var body placeBidRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Amount <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_request",
			"amount must be a positive whole number.", nil, middleware.RequestIDFromContext(r.Context()))
		return
	}

	result, err := h.store.PlaceBid(r.Context(), id, sessionToken, body.Amount)
	if err != nil {
		writeError(w, r, err)
		return
	}
	logBidAccepted(r, "bid accepted", id, result)
	writeBidAccept(w, result)
}

// BuyNow handles POST /v1/listings/{id}/buy-now -- no request body, no amount
// (a listing has exactly one buy-now price).
func (h *Bids) BuyNow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseListingID(w, r)
	if !ok {
		return
	}
	sessionToken := sessionTokenOf(r)

	if !h.checkRateLimit(w, r, sessionToken) {
		return
	}

	result, err := h.store.BuyNow(r.Context(), id, sessionToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	logBidAccepted(r, "buy-now accepted", id, result)
	writeBidAccept(w, result)
}

// checkRateLimit writes the 429 response itself and reports false when the
// caller should stop; true means the request is clear to proceed.
func (h *Bids) checkRateLimit(w http.ResponseWriter, r *http.Request, sessionToken string) bool {
	allowed, err := h.rateLimiter.Allow(r.Context(), sessionToken, bidRateLimitBucket)
	if err != nil {
		writeError(w, r, err)
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", bidRateLimitRetryAfterSeconds)
		httputil.WriteError(w, http.StatusTooManyRequests, "rate_limited",
			"Too many bid attempts -- please slow down.", nil, middleware.RequestIDFromContext(r.Context()))
		return false
	}
	return true
}

// logBidAccepted is the correlation point between an operational log line and
// the durable audit trail (guidelines/06-backend-architecture.md, "Logging"):
// bid_id is the same value stored as bids.id (it round-trips unchanged from
// here through the Lua accept path, the Redis stream, and the stream tailer's
// insert), so this line plus that row are enough to trace a request all the
// way to its durable record -- without needing a dedicated bids.request_id
// column populated on every write path just to duplicate what bid_id already
// gives for free.
func logBidAccepted(r *http.Request, msg, listingID string, result bidding.Result) {
	logging.FromContext(r.Context()).Info(msg,
		"listing_id", listingID, "bid_id", result.BidID, "current_bid", result.CurrentPrice)
}

func writeBidAccept(w http.ResponseWriter, result bidding.Result) {
	httputil.WriteJSON(w, http.StatusCreated, dto.BidAcceptResponse{Data: dto.BidAccept{
		BidID:      result.BidID,
		CurrentBid: result.CurrentPrice,
		BidCount:   result.BidCount,
		AcceptedAt: result.AcceptedAt.UTC().Format(time.RFC3339),
		Viewer:     dto.AcceptedByCaller,
	}})
}
