//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
)

func startRedis(t *testing.T) *goredis.Client {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate redis container: %v", err)
		}
	})

	connStr, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}
	opts, err := goredis.ParseURL(connStr)
	if err != nil {
		t.Fatalf("parse redis url %q: %v", connStr, err)
	}

	client := goredis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	return client
}

// primeListingState writes the {listing:<id>}:state hash fields the Lua accept
// path reads, mirroring what cmd/reconcile's Redis-priming pass will do once it
// exists (a later commit) -- setting these up directly here is the honest thing
// to do for a test that's specifically about the Lua script's own logic, not
// about reconcile.
func primeListingState(t *testing.T, rdb *goredis.Client, listingID string, currentPrice int64, bidCount int, start, end time.Time) {
	t.Helper()
	ctx := context.Background()
	err := rdb.HSet(ctx, redisstore.ListingStateKey(listingID),
		"current_price", currentPrice,
		"bid_count", bidCount,
		"auction_start_ms", start.UnixMilli(),
		"auction_end_ms", end.UnixMilli(),
		"version", 0,
		"high_bidder_session", "",
	).Err()
	if err != nil {
		t.Fatalf("prime listing state: %v", err)
	}
}

func TestBidStore_PlaceBid(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)

	now := time.Now()
	activeStart, activeEnd := now.Add(-time.Hour), now.Add(time.Hour)

	t.Run("accepts a bid at exactly the minimum", func(t *testing.T) {
		listingID := "listing-min-bid"
		primeListingState(t, rdb, listingID, 1_000, 0, activeStart, activeEnd)

		res, err := store.PlaceBid(ctx, listingID, "session-a", 1_100) // 1000 + tier1 increment (100)
		if err != nil {
			t.Fatalf("PlaceBid: %v", err)
		}
		if res.CurrentPrice != 1_100 || res.BidCount != 1 {
			t.Errorf("Result = %+v, want CurrentPrice=1100 BidCount=1", res)
		}
	})

	t.Run("rejects a bid below the minimum, with the minimum in Details", func(t *testing.T) {
		listingID := "listing-too-low"
		primeListingState(t, rdb, listingID, 1_000, 0, activeStart, activeEnd)

		_, err := store.PlaceBid(ctx, listingID, "session-a", 1_099)
		if !errors.Is(err, domain.ErrBidTooLow) {
			t.Fatalf("err = %v, want domain.ErrBidTooLow", err)
		}
		var de *domain.DomainError
		if errors.As(err, &de) {
			// int64, not float64: scriptResult.Minimum is a typed int64 struct field,
			// populated by json.Unmarshal directly -- there's no untyped-JSON-number
			// step in between where it could have become a float64.
			if got := de.Details["minimum"]; got != int64(1_100) {
				t.Errorf("Details[minimum] = %v (%T), want int64(1100)", got, got)
			}
		} else {
			t.Error("errors.As(err, *domain.DomainError) = false, want true")
		}
	})

	t.Run("rejects a bid on a listing that hasn't started", func(t *testing.T) {
		listingID := "listing-not-started"
		primeListingState(t, rdb, listingID, 1_000, 0, now.Add(time.Hour), now.Add(2*time.Hour))

		_, err := store.PlaceBid(ctx, listingID, "session-a", 1_100)
		if !errors.Is(err, domain.ErrAuctionNotStarted) {
			t.Errorf("err = %v, want domain.ErrAuctionNotStarted", err)
		}
	})

	t.Run("rejects a bid on a listing that has ended", func(t *testing.T) {
		listingID := "listing-ended"
		primeListingState(t, rdb, listingID, 1_000, 0, now.Add(-2*time.Hour), now.Add(-time.Hour))

		_, err := store.PlaceBid(ctx, listingID, "session-a", 1_100)
		if !errors.Is(err, domain.ErrAuctionEnded) {
			t.Errorf("err = %v, want domain.ErrAuctionEnded", err)
		}
	})

	t.Run("rejects a bid on an unknown listing", func(t *testing.T) {
		_, err := store.PlaceBid(ctx, "listing-does-not-exist", "session-a", 1_100)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want domain.ErrNotFound", err)
		}
	})

	t.Run("an identical retry replays the original result instead of re-evaluating", func(t *testing.T) {
		listingID := "listing-idempotent"
		primeListingState(t, rdb, listingID, 1_000, 0, activeStart, activeEnd)

		first, err := store.PlaceBid(ctx, listingID, "session-a", 1_100)
		if err != nil {
			t.Fatalf("first PlaceBid: %v", err)
		}
		second, err := store.PlaceBid(ctx, listingID, "session-a", 1_100)
		if err != nil {
			t.Fatalf("second (retried) PlaceBid: %v", err)
		}
		if second.BidID != first.BidID {
			t.Errorf("retried call got a different BidID (%s) than the original (%s) -- it was re-evaluated, not replayed", second.BidID, first.BidID)
		}
		if second.BidCount != 1 {
			t.Errorf("BidCount after an identical retry = %d, want 1 (must not double-count)", second.BidCount)
		}
	})

	t.Run("an accepted bid is recorded in the session's bids set (backs viewer.has_bid)", func(t *testing.T) {
		listingID := "listing-session-set"
		primeListingState(t, rdb, listingID, 1_000, 0, activeStart, activeEnd)

		if _, err := store.PlaceBid(ctx, listingID, "session-b", 1_100); err != nil {
			t.Fatalf("PlaceBid: %v", err)
		}

		isMember, err := rdb.SIsMember(ctx, redisstore.SessionBidsKey("session-b"), listingID).Result()
		if err != nil {
			t.Fatalf("SIsMember: %v", err)
		}
		if !isMember {
			t.Error("expected the listing id to be a member of session-b's bids set after an accepted bid")
		}
	})

	t.Run("an accepted bid is appended to the listing's stream", func(t *testing.T) {
		listingID := "listing-stream"
		primeListingState(t, rdb, listingID, 1_000, 0, activeStart, activeEnd)

		if _, err := store.PlaceBid(ctx, listingID, "session-a", 1_100); err != nil {
			t.Fatalf("PlaceBid: %v", err)
		}

		length, err := rdb.XLen(ctx, redisstore.ListingStreamKey(listingID)).Result()
		if err != nil {
			t.Fatalf("XLen: %v", err)
		}
		if length != 1 {
			t.Errorf("stream length = %d, want 1", length)
		}
	})

	t.Run("successive bids must each clear the next tier's minimum", func(t *testing.T) {
		listingID := "listing-successive"
		primeListingState(t, rdb, listingID, 4_950, 0, activeStart, activeEnd) // just under the $5,000 tier boundary

		// 4950 + 100 (tier1) = 5050, which crosses into tier2 (>= 5000) for the *next* bid.
		first, err := store.PlaceBid(ctx, listingID, "session-a", 5_050)
		if err != nil {
			t.Fatalf("first PlaceBid: %v", err)
		}
		if first.CurrentPrice != 5_050 {
			t.Fatalf("CurrentPrice = %d, want 5050", first.CurrentPrice)
		}

		// Now current_price=5050 (tier2, +250), so 5100 (only +50) must be rejected.
		if _, err := store.PlaceBid(ctx, listingID, "session-b", 5_100); !errors.Is(err, domain.ErrBidTooLow) {
			t.Errorf("err = %v, want domain.ErrBidTooLow (5100 is below the tier2 minimum of 5300)", err)
		}

		second, err := store.PlaceBid(ctx, listingID, "session-b", 5_300)
		if err != nil {
			t.Fatalf("second PlaceBid: %v", err)
		}
		if second.BidCount != 2 {
			t.Errorf("BidCount = %d, want 2", second.BidCount)
		}
	})
}

// TestBidStore_BidListingIDs covers bidding.ViewerLookup -- the read side of
// the same set place_bid.lua SADDs into on every acceptance (tested above via
// raw SIsMember calls; this is the actual production method handlers call).
func TestBidStore_BidListingIDs(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)

	now := time.Now()
	activeStart, activeEnd := now.Add(-time.Hour), now.Add(time.Hour)
	listingA, listingB := "listing-viewer-a", "listing-viewer-b"
	primeListingState(t, rdb, listingA, 1_000, 0, activeStart, activeEnd)
	primeListingState(t, rdb, listingB, 1_000, 0, activeStart, activeEnd)

	t.Run("a session with no accepted bids at all gets an empty set, not an error", func(t *testing.T) {
		ids, err := store.BidListingIDs(ctx, "session-never-bid")
		if err != nil {
			t.Fatalf("BidListingIDs: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("ids = %v, want empty", ids)
		}
	})

	t.Run("accepted bids across multiple listings all show up, and only for that session", func(t *testing.T) {
		if _, err := store.PlaceBid(ctx, listingA, "session-viewer", 1_100); err != nil {
			t.Fatalf("PlaceBid(listingA): %v", err)
		}
		if _, err := store.PlaceBid(ctx, listingB, "session-viewer", 1_100); err != nil {
			t.Fatalf("PlaceBid(listingB): %v", err)
		}
		if _, err := store.PlaceBid(ctx, listingA, "session-other", 1_200); err != nil {
			t.Fatalf("PlaceBid(listingA, other session): %v", err)
		}

		ids, err := store.BidListingIDs(ctx, "session-viewer")
		if err != nil {
			t.Fatalf("BidListingIDs: %v", err)
		}
		if _, ok := ids[listingA]; !ok {
			t.Error("expected listingA in session-viewer's bid set")
		}
		if _, ok := ids[listingB]; !ok {
			t.Error("expected listingB in session-viewer's bid set")
		}
		if len(ids) != 2 {
			t.Errorf("ids = %v, want exactly 2 entries (not session-other's bid too)", ids)
		}
	})
}

// Distinct from "an identical retry replays the original result" above: this
// proves the *other* half of that behavior -- once the idempotency cache
// entry's own TTL has actually elapsed, the identical (session, listing, amount)
// triple is evaluated fresh again, not replayed forever. Both halves matter: a
// cache that never expired would still pass every other bidding test here, so
// this needs its own real wall-clock check against Redis.
func TestBidStore_PlaceBid_IdempotencyExpiry(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	// Redis's SET ... EX only accepts whole seconds, so 1s is as short as this
	// can be made (matching the same floor already hit in the session store test).
	store := redisstore.NewBidStore(rdb, time.Second)

	listingID := "listing-idempotency-expiry"
	now := time.Now()
	primeListingState(t, rdb, listingID, 1_000, 0, now.Add(-time.Hour), now.Add(time.Hour))

	if _, err := store.PlaceBid(ctx, listingID, "session-a", 1_100); err != nil {
		t.Fatalf("first PlaceBid: %v", err)
	}

	time.Sleep(1200 * time.Millisecond) // past the 1s idempotency TTL

	// The identical (session, listing, amount) triple is re-evaluated for real
	// this time, not replayed -- and since current_price already moved to 1100,
	// bidding 1100 again now correctly fails as too low.
	_, err := store.PlaceBid(ctx, listingID, "session-a", 1_100)
	if !errors.Is(err, domain.ErrBidTooLow) {
		t.Fatalf("after idempotency expiry, err = %v, want domain.ErrBidTooLow (re-evaluated, not replayed)", err)
	}

	finalCount, err := rdb.HGet(ctx, redisstore.ListingStateKey(listingID), "bid_count").Int()
	if err != nil {
		t.Fatalf("read bid_count: %v", err)
	}
	if finalCount != 1 {
		t.Errorf("bid_count = %d, want 1 (the second call was correctly rejected, not counted)", finalCount)
	}
}
