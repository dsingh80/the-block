//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/inmemory"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// fakeSubscriber records every event it's notified of, safe for concurrent use.
// (A separate type from inmemory's own test-only fakeSubscriber -- that one
// isn't importable from outside its package.)
type fakeSubscriber struct {
	mu     sync.Mutex
	events []realtime.Event
}

func (f *fakeSubscriber) Notify(e realtime.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeSubscriber) received() []realtime.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]realtime.Event, len(f.events))
	copy(out, f.events)
	return out
}

// TestStreamTailer_DrainAndRestart runs the whole real pipeline together --
// BidStore accepting bids in Redis, StreamTailer draining them into Postgres and
// broadcasting them -- and specifically exercises a simulated tailer restart
// mid-stream, which is the scenario the checkpoint mechanism exists for.
func TestStreamTailer_DrainAndRestart(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)

	listing := sampleListing("77777777-7777-7777-7777-777777777777", "VINTAILER1")
	// sampleListing's fixed 2026-07-28 AuctionStart is a fixture date for
	// pgstore-only tests that don't check lifecycle -- BidStore's Lua path
	// does, so this needs a window that's genuinely active right now.
	listing.AuctionStart = time.Now().Add(-time.Hour)
	listing.AuctionDuration = 24 * time.Hour
	buyNowPrice := int64(50_000)
	listing.BuyNowPrice = &buyNowPrice
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}
	reader := pgstore.NewListingReader(pool)
	fromPostgres, err := reader.Get(ctx, listing.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := redisstore.PrimeListingState(ctx, rdb, fromPostgres); err != nil {
		t.Fatalf("PrimeListingState: %v", err)
	}

	bidStore := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	broadcaster1 := inmemory.NewBroadcaster()
	sub := &fakeSubscriber{}
	broadcaster1.Subscribe(sub, listing.ID)
	tailer1 := redisstore.NewStreamTailer(rdb, pool, broadcaster1)

	// current_price starts at 20,500 -- already >= the $15,000 tier-3 floor, so
	// each bid must clear a +$500 increment, not +$100 (tier 1 only applies
	// below $5,000). 21000 -> 21500 -> 22000 -> 22500.

	// --- Phase 1: three ordinary bids, then one drain pass ---
	if _, err := bidStore.PlaceBid(ctx, listing.ID, "session-a", 21_000); err != nil {
		t.Fatalf("bid 1: %v", err)
	}
	if _, err := bidStore.PlaceBid(ctx, listing.ID, "session-b", 21_500); err != nil {
		t.Fatalf("bid 2: %v", err)
	}
	if _, err := bidStore.PlaceBid(ctx, listing.ID, "session-a", 22_000); err != nil {
		t.Fatalf("bid 3: %v", err)
	}

	if err := tailer1.Tick(ctx); err != nil {
		t.Fatalf("Tick (phase 1): %v", err)
	}

	assertBidRows(t, ctx, pool, listing.ID, 3, []int64{21_000, 21_500, 22_000})
	assertListingRow(t, ctx, pool, listing.ID, 22_000, 3, "session-a")

	events1 := sub.received()
	if len(events1) != 3 {
		t.Fatalf("subscriber received %d events after phase 1, want 3", len(events1))
	}
	for i, want := range []int64{21_000, 21_500, 22_000} {
		if events1[i].Type != "bid_accepted" || events1[i].CurrentPrice != want {
			t.Errorf("event %d = %+v, want bid_accepted at %d", i, events1[i], want)
		}
	}

	// --- Phase 2: simulate a process restart. A brand-new StreamTailer (and a
	// fresh Broadcaster, as a real restarted process would have) reads the same
	// Postgres checkpoint. One more ordinary bid, then a Buy Now. ---
	if _, err := bidStore.PlaceBid(ctx, listing.ID, "session-b", 22_500); err != nil {
		t.Fatalf("bid 4: %v", err)
	}
	if _, err := bidStore.BuyNow(ctx, listing.ID, "session-a"); err != nil {
		t.Fatalf("buy now: %v", err)
	}

	broadcaster2 := inmemory.NewBroadcaster()
	sub2 := &fakeSubscriber{}
	broadcaster2.Subscribe(sub2, listing.ID)
	tailer2 := redisstore.NewStreamTailer(rdb, pool, broadcaster2) // a genuinely new instance -- no in-memory state from tailer1

	if err := tailer2.Tick(ctx); err != nil {
		t.Fatalf("Tick (phase 2, post-restart): %v", err)
	}

	// Total rows must be exactly 5 -- the 3 from phase 1 plus these 2, no
	// duplicates from re-reading anything phase 1 already drained.
	assertBidRows(t, ctx, pool, listing.ID, 5, []int64{21_000, 21_500, 22_000, 22_500, 50_000})
	assertListingRow(t, ctx, pool, listing.ID, 50_000, 5, "session-a")

	var purchasedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT purchased_at FROM listings WHERE id = $1`, listing.ID).Scan(&purchasedAt); err != nil {
		t.Fatalf("query purchased_at: %v", err)
	}
	if purchasedAt == nil {
		t.Error("purchased_at is still null after a drained buy-now entry")
	}

	events2 := sub2.received()
	if len(events2) != 2 {
		t.Fatalf("post-restart subscriber received %d events, want exactly 2 (no re-delivery of phase 1's 3)", len(events2))
	}
	if events2[0].Type != "bid_accepted" || events2[0].CurrentPrice != 22_500 {
		t.Errorf("event2[0] = %+v, want bid_accepted at 22500", events2[0])
	}
	if events2[1].Type != "listing_ended" || events2[1].Reason != "bought_now" || events2[1].CurrentPrice != 50_000 {
		t.Errorf("event2[1] = %+v, want listing_ended/bought_now at 50000", events2[1])
	}

	// A third tick (same as any idle poll) with nothing new must be a clean no-op.
	if err := tailer2.Tick(ctx); err != nil {
		t.Fatalf("Tick (idle, no new entries): %v", err)
	}
	assertBidRows(t, ctx, pool, listing.ID, 5, []int64{21_000, 21_500, 22_000, 22_500, 50_000})
}

func TestStreamTailer_Tick_EmptyDatabaseIsANoOp(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t) // migrated, but zero listings inserted
	rdb := startRedis(t)

	tailer := redisstore.NewStreamTailer(rdb, pool, inmemory.NewBroadcaster())
	if err := tailer.Tick(ctx); err != nil {
		t.Errorf("Tick() on an empty listings table = %v, want nil", err)
	}
}

func TestStreamTailer_Tick_CanceledContextReturnsWrappedError(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)

	listing := sampleListing("88888888-8888-8888-8888-888888888888", "VINTAILERCANCEL")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	tailer := redisstore.NewStreamTailer(rdb, pool, inmemory.NewBroadcaster())
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := tailer.Tick(canceledCtx); err == nil {
		t.Error("expected an error for Tick called with an already-canceled context, got nil")
	}
}

// TestStreamTailer_Tick_MalformedStreamEntryIsLoggedAndSkipped covers
// processEntry's field-parsing errors -- unexported, so only reachable through
// Tick itself. A malformed entry can only get onto a stream via a bug in the
// Lua accept path (never through this test's normal BidStore calls), so this
// writes one directly with XAdd to simulate that. Tick must not crash or
// propagate the per-entry error (a good ~200 other listings' entries in the
// same tick shouldn't be lost over one bad entry) -- it should log and move on,
// leaving no partial/corrupt row behind for the entry that failed to parse.
func TestStreamTailer_Tick_MalformedStreamEntryIsLoggedAndSkipped(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)

	listing := sampleListing("99999999-8888-8888-8888-888888888888", "VINTAILERBAD")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redisstore.ListingStreamKey(listing.ID),
		Values: map[string]any{
			"bid_id": "bad-bid-1", "type": "bid", "session_id": "session-a",
			"amount": "not-a-number", "bid_count": "1", "accepted_at_ms": time.Now().UnixMilli(),
		},
	}).Err()
	if err != nil {
		t.Fatalf("XAdd malformed entry: %v", err)
	}

	tailer := redisstore.NewStreamTailer(rdb, pool, inmemory.NewBroadcaster())
	if err := tailer.Tick(ctx); err != nil {
		t.Fatalf("Tick() = %v, want nil (a per-entry parse failure must be logged and skipped, not propagated)", err)
	}

	assertBidRows(t, ctx, pool, listing.ID, 0, nil)
}

func assertBidRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, listingID string, wantCount int, wantAmountsInOrder []int64) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT amount FROM bids WHERE listing_id = $1 ORDER BY accepted_at, id`, listingID)
	if err != nil {
		t.Fatalf("query bids: %v", err)
	}
	defer rows.Close()

	var amounts []int64
	for rows.Next() {
		var a int64
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan bid amount: %v", err)
		}
		amounts = append(amounts, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate bids: %v", err)
	}

	if len(amounts) != wantCount {
		t.Fatalf("bids row count = %d, want %d (amounts: %v)", len(amounts), wantCount, amounts)
	}
	for i, want := range wantAmountsInOrder {
		if amounts[i] != want {
			t.Errorf("bids[%d].amount = %d, want %d (full order: %v)", i, amounts[i], want, amounts)
		}
	}
}

func assertListingRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, listingID string, wantPrice int64, wantBidCount int, wantHighBidder string) {
	t.Helper()
	var price int64
	var count int
	var highBidder *string
	err := pool.QueryRow(ctx, `SELECT current_price, bid_count, high_bidder_session_id FROM listings WHERE id = $1`, listingID).
		Scan(&price, &count, &highBidder)
	if err != nil {
		t.Fatalf("query listing row: %v", err)
	}
	if price != wantPrice || count != wantBidCount {
		t.Errorf("listing current_price/bid_count = %d/%d, want %d/%d", price, count, wantPrice, wantBidCount)
	}
	if highBidder == nil || *highBidder != wantHighBidder {
		t.Errorf("listing high_bidder_session_id = %v, want %q", highBidder, wantHighBidder)
	}
}
