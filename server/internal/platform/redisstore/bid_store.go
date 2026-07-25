package redisstore

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
)

//go:embed scripts/place_bid.lua
var placeBidSource string

//go:embed scripts/buy_now.lua
var buyNowSource string

// redis.NewScript gives EVALSHA-with-NOSCRIPT-fallback for free (it tries EVALSHA
// first, and on a NOSCRIPT reply transparently EVALs and caches it) -- no need to
// hand-rolled SCRIPT LOAD bookkeeping ourselves.
var placeBidScript = redis.NewScript(placeBidSource)
var buyNowScript = redis.NewScript(buyNowSource)

// DefaultIdempotencyTTL bounds how long a (session, listing, amount) triple's
// cached result is replayed -- long enough to cover a realistic retry delay,
// short enough not to accumulate stale entries forever
// (guidelines/06-backend-architecture.md). Production code uses this constant;
// tests pass a much shorter TTL via NewBidStore's explicit parameter so cache
// expiry can be observed without a real 10-minute wait.
const DefaultIdempotencyTTL = 10 * time.Minute

// BidStore implements bidding.Store against Redis's Lua-scripted accept path
// (guidelines/06-backend-architecture.md).
type BidStore struct {
	rdb            *redis.Client
	idempotencyTTL time.Duration
}

func NewBidStore(rdb *redis.Client, idempotencyTTL time.Duration) *BidStore {
	return &BidStore{rdb: rdb, idempotencyTTL: idempotencyTTL}
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
		amount, sessionID, bidID.String(), int(s.idempotencyTTL.Seconds()),
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

// BuyNow is place_bid.lua's structural sibling: same idempotency/existence/
// lifecycle checks, no amount (a listing has exactly one buy-now price).
func (s *BidStore) BuyNow(ctx context.Context, listingID, sessionID string) (bidding.Result, error) {
	bidID, err := uuid.NewV7()
	if err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: generate bid id: %w", err)
	}

	keys := []string{
		ListingStateKey(listingID),
		ListingStreamKey(listingID),
		IdempotencyBuyNowKey(sessionID, listingID),
		SessionBidsKey(sessionID),
	}
	args := []any{sessionID, bidID.String(), int(s.idempotencyTTL.Seconds()), listingID}

	raw, err := buyNowScript.Run(ctx, s.rdb, keys, args...).Text()
	if err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: buy_now script: %w", err)
	}

	var res scriptResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return bidding.Result{}, fmt.Errorf("redisstore: decode buy_now result %q: %w", raw, err)
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

// BidListingIDs implements bidding.ViewerLookup: the listing ids sessionToken
// has an accepted bid on, from the same set place_bid.lua/buy_now.lua SADD on
// every acceptance (guidelines/06-backend-architecture.md). One SMEMBERS call
// regardless of how many listings the caller is about to render viewer fields
// for -- membership is then a plain map lookup per row, not a further round trip.
func (s *BidStore) BidListingIDs(ctx context.Context, sessionToken string) (map[string]struct{}, error) {
	ids, err := s.rdb.SMembers(ctx, SessionBidsKey(sessionToken)).Result()
	if err != nil {
		return nil, fmt.Errorf("redisstore: list bid listing ids for session %s: %w", sessionToken, err)
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// HighBidderSessions implements bidding.ViewerLookup: one pipelined HGET of
// high_bidder_session per listing id, in a single round trip regardless of
// how many ids are asked for. Callers only ever ask this for the small set of
// listings actually ambiguous for the requesting session (has_bid true but
// not the Postgres-recorded high bidder -- see domain.Viewer.ReconcileHighBidder),
// not every listing on a page, so this stays cheap in practice.
func (s *BidStore) HighBidderSessions(ctx context.Context, listingIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(listingIDs))
	if len(listingIDs) == 0 {
		return result, nil
	}

	pipe := s.rdb.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(listingIDs))
	for _, id := range listingIDs {
		cmds[id] = pipe.HGet(ctx, ListingStateKey(id), "high_bidder_session")
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("redisstore: high bidder sessions: %w", err)
	}

	for id, cmd := range cmds {
		v, err := cmd.Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue // no state hash (or field) for this id -- no high bidder yet
			}
			return nil, fmt.Errorf("redisstore: high bidder session for %s: %w", id, err)
		}
		if v != "" {
			result[id] = v
		}
	}
	return result, nil
}

// scriptErrorToDomain maps place_bid.lua/buy_now.lua's `error` code verbatim onto
// the same domain.DomainError vocabulary the HTTP error envelope surfaces
// (guidelines/06-backend-architecture.md) -- no separate translation table
// between the atomic path and HTTP.
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
	case "buy_now_unavailable":
		return domain.ErrBuyNowUnavailable
	default:
		return fmt.Errorf("redisstore: unrecognized script error code %q", res.Error)
	}
}
