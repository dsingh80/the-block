// Package bidding is the bidding use-case: the atomic accept-path port HTTP
// handlers depend on, implemented by internal/platform/redisstore's Lua scripts
// (guidelines/06-backend-architecture.md). Kept free of net/http and redis types.
package bidding

import (
	"context"
	"time"
)

// Result is what an accepted bid or buy-now resolves to. A rejection is a
// *domain.DomainError, not a zero Result -- see guidelines/06-backend-architecture.md's
// "Go conventions" (typed results at use-case boundaries, not panics).
type Result struct {
	BidID        string
	CurrentPrice int64
	BidCount     int
	AcceptedAt   time.Time
}

// Store is the atomic bid-accept port.
type Store interface {
	PlaceBid(ctx context.Context, listingID, sessionID string, amount int64) (Result, error)
	BuyNow(ctx context.Context, listingID, sessionID string) (Result, error)
}

// ViewerLookup answers which listings a session has an accepted bid on -- the
// join-free half of computing a listing's `viewer` object
// (guidelines/06-backend-architecture.md, "Computing viewer without joins").
// Populated by the same accept path as Store: place_bid.lua/buy_now.lua SADD
// the session's own bids set on every acceptance, so this is a separate,
// narrower interface over the same underlying store, not a different one.
type ViewerLookup interface {
	BidListingIDs(ctx context.Context, sessionToken string) (map[string]struct{}, error)

	// HighBidderSessions answers the *live* high_bidder_session for each given
	// listing id that has one set -- the same Redis field the accept path
	// writes atomically alongside the SADD that backs BidListingIDs, so unlike
	// a listing's Postgres-derived HighBidderSessionID it can never lag a
	// session's own just-accepted bid (see domain.Viewer.ReconcileHighBidder).
	// A listing with no entry in the returned map has no high bidder yet.
	HighBidderSessions(ctx context.Context, listingIDs []string) (map[string]string, error)
}

// RateLimiter mitigates bid-attempt abuse without needing real auth first
// (guidelines/06-backend-architecture.md, "SOC2 principles mapping"). Allow
// reports whether this (sessionToken, bucket) request is within its window.
type RateLimiter interface {
	Allow(ctx context.Context, sessionToken, bucket string) (bool, error)
}
