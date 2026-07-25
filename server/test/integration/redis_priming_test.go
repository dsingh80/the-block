//go:build integration

package integration

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
)

func TestPrimeListingState(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)
	reader := pgstore.NewListingReader(pool)

	listing := sampleListing("55555555-5555-5555-5555-555555555555", "VINPRIME01")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}
	fromPostgres, err := reader.Get(ctx, listing.ID) // has the trigger-computed AuctionEnd
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	t.Run("priming a new listing sets every field from Postgres", func(t *testing.T) {
		if err := redisstore.PrimeListingState(ctx, rdb, fromPostgres); err != nil {
			t.Fatalf("PrimeListingState: %v", err)
		}

		key := redisstore.ListingStateKey(listing.ID)
		if price, err := rdb.HGet(ctx, key, "current_price").Int64(); err != nil || price != 20_500 { // sampleListing's CurrentPrice
			t.Errorf("current_price = %d (err=%v), want 20500", price, err)
		}
		if count, err := rdb.HGet(ctx, key, "bid_count").Int(); err != nil || count != 0 {
			t.Errorf("bid_count = %d (err=%v), want 0", count, err)
		}
		wantEndMs := fromPostgres.AuctionEnd.UnixMilli()
		if endMs, err := rdb.HGet(ctx, key, "auction_end_ms").Int64(); err != nil || endMs != wantEndMs {
			t.Errorf("auction_end_ms = %d (err=%v), want %d", endMs, err, wantEndMs)
		}
	})

	t.Run("a real bid's state is never overwritten by re-priming", func(t *testing.T) {
		key := redisstore.ListingStateKey(listing.ID)

		// Simulate a real accepted bid having moved the price in Redis, as
		// place_bid.lua would.
		if err := rdb.HSet(ctx, key, "current_price", 55_000, "bid_count", 9).Err(); err != nil {
			t.Fatalf("simulate a live bid: %v", err)
		}

		// Reconcile runs again (e.g. the container restarted) with the same
		// Postgres-sourced listing, whose CurrentPrice is still the original 20500.
		if err := redisstore.PrimeListingState(ctx, rdb, fromPostgres); err != nil {
			t.Fatalf("re-prime: %v", err)
		}

		vals, err := rdb.HMGet(ctx, key, "current_price", "bid_count").Result()
		if err != nil {
			t.Fatalf("HMGet: %v", err)
		}
		if vals[0] != "55000" || vals[1] != "9" {
			t.Errorf("current_price/bid_count = %v/%v, want 55000/9 (the live bid must survive re-priming)", vals[0], vals[1])
		}
	})

	t.Run("Redis-data-loss recovery restores from whatever Postgres already reflects", func(t *testing.T) {
		listing2 := sampleListing("66666666-6666-6666-6666-666666666666", "VINPRIME02")
		if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing2}); err != nil {
			t.Fatalf("InsertNewListings: %v", err)
		}

		// A real bid already happened and was durably recorded in Postgres...
		if _, err := pool.Exec(ctx, `UPDATE listings SET current_price = 41_000, bid_count = 3 WHERE id = $1`, listing2.ID); err != nil {
			t.Fatalf("simulate a durably-recorded bid: %v", err)
		}
		recovered, err := reader.Get(ctx, listing2.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		// ...but Redis has never seen this listing at all (simulating Redis
		// having lost its data, e.g. a restart with no AOF).
		key := redisstore.ListingStateKey(listing2.ID)
		if err := redisstore.PrimeListingState(ctx, rdb, recovered); err != nil {
			t.Fatalf("PrimeListingState (recovery): %v", err)
		}

		vals, err := rdb.HMGet(ctx, key, "current_price", "bid_count").Result()
		if err != nil {
			t.Fatalf("HMGet: %v", err)
		}
		if vals[0] != "41000" || vals[1] != "3" {
			t.Errorf("current_price/bid_count = %v/%v, want 41000/3 (recovered from Postgres, not reset to starting_bid/0)", vals[0], vals[1])
		}
	})

	t.Run("buy_now_price, purchased_at, and a high bidder are only primed when actually set", func(t *testing.T) {
		buyNowPrice := int64(35_000)
		highBidder := "session-high-bidder"
		purchasedAt := time.Now().Add(-time.Minute)
		l := sampleListing("77777777-5555-5555-5555-555555555555", "VINPRIME03")
		l.BuyNowPrice = &buyNowPrice
		l.HighBidderSessionID = &highBidder
		l.PurchasedAt = &purchasedAt

		key := redisstore.ListingStateKey(l.ID)
		if err := redisstore.PrimeListingState(ctx, rdb, l); err != nil {
			t.Fatalf("PrimeListingState: %v", err)
		}

		vals, err := rdb.HMGet(ctx, key, "buy_now_price", "high_bidder_session", "purchased_at_ms").Result()
		if err != nil {
			t.Fatalf("HMGet: %v", err)
		}
		if vals[0] != "35000" {
			t.Errorf("buy_now_price = %v, want 35000", vals[0])
		}
		if vals[1] != highBidder {
			t.Errorf("high_bidder_session = %v, want %q", vals[1], highBidder)
		}
		wantPurchasedAtMs := strconv.FormatInt(purchasedAt.UnixMilli(), 10)
		if vals[2] != wantPurchasedAtMs {
			t.Errorf("purchased_at_ms = %v, want %s", vals[2], wantPurchasedAtMs)
		}
	})

	t.Run("a canceled context surfaces as a wrapped error, not a hang", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := redisstore.PrimeListingState(canceledCtx, rdb, fromPostgres); err == nil {
			t.Error("expected an error for PrimeListingState called with an already-canceled context, got nil")
		}
	})
}
