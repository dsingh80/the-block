//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
)

func setBuyNowPrice(t *testing.T, rdb *goredis.Client, listingID string, price int64) {
	t.Helper()
	if err := rdb.HSet(context.Background(), redisstore.ListingStateKey(listingID), "buy_now_price", price).Err(); err != nil {
		t.Fatalf("set buy_now_price: %v", err)
	}
}

func TestBidStore_BuyNow(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)

	now := time.Now()
	activeStart, activeEnd := now.Add(-time.Hour), now.Add(time.Hour)

	t.Run("accepts when current price is below the buy-now price", func(t *testing.T) {
		listingID := "buynow-accept"
		primeListingState(t, rdb, listingID, 20_500, 3, activeStart, activeEnd)
		setBuyNowPrice(t, rdb, listingID, 32_000)

		res, err := store.BuyNow(ctx, listingID, "session-a")
		if err != nil {
			t.Fatalf("BuyNow: %v", err)
		}
		if res.CurrentPrice != 32_000 || res.BidCount != 4 {
			t.Errorf("Result = %+v, want CurrentPrice=32000 BidCount=4", res)
		}

		purchasedAt, err := rdb.HGet(ctx, redisstore.ListingStateKey(listingID), "purchased_at_ms").Result()
		if err != nil || purchasedAt == "" {
			t.Errorf("purchased_at_ms not set after a successful buy-now (err=%v, value=%q)", err, purchasedAt)
		}
	})

	t.Run("rejects once the current price already meets or exceeds the buy-now price", func(t *testing.T) {
		listingID := "buynow-already-exceeded"
		primeListingState(t, rdb, listingID, 32_000, 5, activeStart, activeEnd) // a rival bid already reached it
		setBuyNowPrice(t, rdb, listingID, 32_000)

		_, err := store.BuyNow(ctx, listingID, "session-a")
		if !errors.Is(err, domain.ErrBuyNowUnavailable) {
			t.Errorf("err = %v, want domain.ErrBuyNowUnavailable", err)
		}
	})

	t.Run("rejects when there is no buy-now price at all", func(t *testing.T) {
		listingID := "buynow-none"
		primeListingState(t, rdb, listingID, 20_500, 0, activeStart, activeEnd) // no setBuyNowPrice call

		_, err := store.BuyNow(ctx, listingID, "session-a")
		if !errors.Is(err, domain.ErrBuyNowUnavailable) {
			t.Errorf("err = %v, want domain.ErrBuyNowUnavailable", err)
		}
	})

	t.Run("rejects on an unknown listing", func(t *testing.T) {
		_, err := store.BuyNow(ctx, "buynow-does-not-exist", "session-a")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want domain.ErrNotFound", err)
		}
	})

	t.Run("rejects on a listing that has already ended", func(t *testing.T) {
		listingID := "buynow-ended"
		primeListingState(t, rdb, listingID, 20_500, 0, now.Add(-2*time.Hour), now.Add(-time.Hour))
		setBuyNowPrice(t, rdb, listingID, 32_000)

		_, err := store.BuyNow(ctx, listingID, "session-a")
		if !errors.Is(err, domain.ErrAuctionEnded) {
			t.Errorf("err = %v, want domain.ErrAuctionEnded", err)
		}
	})

	t.Run("an identical retry replays the original result instead of double-buying", func(t *testing.T) {
		listingID := "buynow-idempotent"
		primeListingState(t, rdb, listingID, 20_500, 0, activeStart, activeEnd)
		setBuyNowPrice(t, rdb, listingID, 32_000)

		first, err := store.BuyNow(ctx, listingID, "session-a")
		if err != nil {
			t.Fatalf("first BuyNow: %v", err)
		}
		second, err := store.BuyNow(ctx, listingID, "session-a")
		if err != nil {
			t.Fatalf("second (retried) BuyNow: %v", err)
		}
		if second.BidID != first.BidID {
			t.Errorf("retried call got a different BidID (%s) than the original (%s)", second.BidID, first.BidID)
		}
		if second.BidCount != 1 {
			t.Errorf("BidCount after an identical retry = %d, want 1 (must not double-count)", second.BidCount)
		}
	})
}
