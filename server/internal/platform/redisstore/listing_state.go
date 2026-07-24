package redisstore

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// PrimeListingState ensures {listing:<id>}:state has every field, using HSETNX
// field-by-field so an already-present field is never overwritten -- one code
// path covers both a brand-new listing (fields come from its starting values)
// and a Redis-data-loss recovery scenario (fields come from whatever Postgres
// already reflects, e.g. real bidding that already happened), since either way
// the only fields actually written are the ones still missing
// (guidelines/06-backend-architecture.md, "Data lifecycle"). This is also what
// makes Redis fully disposable/reconstructable in this design: Postgres is the
// only store that has to survive for the system to recover correctly.
func PrimeListingState(ctx context.Context, rdb *redis.Client, l domain.Listing) error {
	key := ListingStateKey(l.ID)

	highBidder := ""
	if l.HighBidderSessionID != nil {
		highBidder = *l.HighBidderSessionID
	}

	fields := map[string]any{
		"current_price":       l.CurrentPrice,
		"bid_count":           l.BidCount,
		"high_bidder_session": highBidder,
		"auction_start_ms":    l.AuctionStart.UnixMilli(),
		"auction_end_ms":      l.AuctionEnd.UnixMilli(),
		"version":             0,
	}
	// Absent entirely (not a sentinel value) means "none" -- place_bid.lua and
	// buy_now.lua already expect this (HMGET returns Lua false/nil for a
	// missing field).
	if l.BuyNowPrice != nil {
		fields["buy_now_price"] = *l.BuyNowPrice
	}
	if l.PurchasedAt != nil {
		fields["purchased_at_ms"] = l.PurchasedAt.UnixMilli()
	}

	for field, value := range fields {
		if err := rdb.HSetNX(ctx, key, field, value).Err(); err != nil {
			return fmt.Errorf("redisstore: prime %s field %s: %w", key, field, err)
		}
	}
	return nil
}
