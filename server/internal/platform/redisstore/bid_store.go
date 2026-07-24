package redisstore

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
)

//go:embed scripts/place_bid.lua
var placeBidSource string

// redis.NewScript gives EVALSHA-with-NOSCRIPT-fallback for free (it tries EVALSHA
// first, and on a NOSCRIPT reply transparently EVALs and caches it) -- no need to
// hand-rolled SCRIPT LOAD bookkeeping ourselves.
var placeBidScript = redis.NewScript(placeBidSource)

// idempotencyTTL bounds how long a (session, listing, amount) triple's cached
// result is replayed -- long enough to cover a realistic retry delay, short
// enough not to accumulate stale entries forever (guidelines/06-backend-architecture.md).
const idempotencyTTL = 10 * time.Minute

// BidStore implements bidding.Store against Redis's Lua-scripted accept path
// (guidelines/06-backend-architecture.md).
type BidStore struct {
	rdb *redis.Client
}

func NewBidStore(rdb *redis.Client) *BidStore {
	return &BidStore{rdb: rdb}
}

// scriptResult mirrors place_bid.lua's cjson.encode(...) shape exactly.
type scriptResult struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error"`
	Minimum      int64  `json:"minimum"`
	BidID        string `json:"bid_id"`
	CurrentPrice int64  `json:"current_price"`
	BidCount     int    `json:"bid_count"`
	AcceptedAtMs int64  `json:"accepted_at_ms"`
}

func (s *BidStore) PlaceBid(ctx context.Context, listingID, sessionID string, amount int64) (bidding.Result, error) {
	bidID, err := uuid.NewV7()
	if err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: generate bid id: %w", err)
	}

	keys := []string{
		ListingStateKey(listingID),
		ListingStreamKey(listingID),
		IdempotencyKey(sessionID, listingID, amount),
		SessionBidsKey(sessionID),
	}
	args := []any{
		amount, sessionID, bidID.String(), int(idempotencyTTL.Seconds()),
		domain.Tier1Ceiling, domain.Tier1Increment,
		domain.Tier2Ceiling, domain.Tier2Increment,
		domain.Tier3Increment,
		listingID,
	}

	raw, err := placeBidScript.Run(ctx, s.rdb, keys, args...).Text()
	if err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: place_bid script: %w", err)
	}

	var res scriptResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: decode place_bid result %q: %w", raw, err)
	}

	if !res.OK {
		return bidding.Result{}, scriptErrorToDomain(res)
	}

	return bidding.Result{
		BidID:        res.BidID,
		CurrentPrice: res.CurrentPrice,
		BidCount:     res.BidCount,
		AcceptedAt:   time.UnixMilli(res.AcceptedAtMs).UTC(),
	}, nil
}

// scriptErrorToDomain maps place_bid.lua's `error` code verbatim onto the same
// domain.DomainError vocabulary the HTTP error envelope surfaces
// (guidelines/06-backend-architecture.md) -- no separate translation table
// between the atomic path and HTTP. buy_now.lua's own error (buy_now_unavailable)
// is added here in the commit that introduces it.
func scriptErrorToDomain(res scriptResult) error {
	switch res.Error {
	case "not_found":
		return domain.ErrNotFound
	case "auction_not_started":
		return domain.ErrAuctionNotStarted
	case "auction_ended":
		return domain.ErrAuctionEnded
	case "bid_too_low":
		return domain.NewBidTooLowError(res.Minimum)
	default:
		return fmt.Errorf("redisstore: unrecognized script error code %q", res.Error)
	}
}
