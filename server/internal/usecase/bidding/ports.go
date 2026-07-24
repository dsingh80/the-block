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
