//go:build integration

// Dedicated concurrency-correctness test for the atomic bid-accept path -- kept in
// its own file/commit, not folded into bidding_test.go, specifically so it can't be
// quietly skipped: this is the empirical proof behind choosing Redis's single-
// threaded script execution for this at all (guidelines/06-backend-architecture.md, D1).
package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
)

// N distinct sessions race to place the *identical* amount -- exactly the amount
// that clears the current minimum -- simultaneously on one listing. Because
// Redis serializes script execution, exactly one of them can be "first" to move
// current_price; every other racer's script invocation runs after that happens
// and sees a price that's already moved, so it's correctly rejected as too low,
// not a duplicate acceptance. This isn't a probabilistic property -- if the
// atomic check-then-write ever let two racers both read the pre-bid price and
// both write an acceptance, this test would show more than one acceptance.
func TestBidStore_PlaceBid_ConcurrencyCorrectness(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	store := redisstore.NewBidStore(rdb)

	const goroutines = 100
	const trials = 5 // repeated with a fresh listing each time -- a race condition can be order-dependent and get lucky once

	for trial := 0; trial < trials; trial++ {
		trial := trial
		t.Run(fmt.Sprintf("trial_%d", trial), func(t *testing.T) {
			listingID := fmt.Sprintf("listing-concurrency-%d", trial)
			now := time.Now()
			primeListingState(t, rdb, listingID, 1_000, 0, now.Add(-time.Hour), now.Add(time.Hour))

			const bidAmount = 1_100 // current_price(1000) + tier1 increment(100) -- the one amount every racer contests

			type outcome struct {
				res bidding.Result
				err error
			}
			results := make([]outcome, goroutines)

			var wg sync.WaitGroup
			wg.Add(goroutines)
			for i := 0; i < goroutines; i++ {
				go func(i int) {
					defer wg.Done()
					sessionID := fmt.Sprintf("session-%d", i)
					res, err := store.PlaceBid(ctx, listingID, sessionID, bidAmount)
					results[i] = outcome{res, err}
				}(i)
			}
			wg.Wait()

			accepted := 0
			for i, r := range results {
				switch {
				case r.err == nil:
					accepted++
				case errors.Is(r.err, domain.ErrBidTooLow):
					// expected for every racer except the one that got there first
				default:
					t.Errorf("racer %d: unexpected error %v", i, r.err)
				}
			}
			if accepted != 1 {
				t.Fatalf("accepted %d of %d identical concurrent bids, want exactly 1", accepted, goroutines)
			}

			// Final Redis state must be internally consistent with "exactly one
			// bid was ever really accepted" -- not just that PlaceBid's return
			// values looked right, but that the underlying data agrees.
			finalPrice, err := rdb.HGet(ctx, redisstore.ListingStateKey(listingID), "current_price").Int64()
			if err != nil {
				t.Fatalf("read final current_price: %v", err)
			}
			if finalPrice != bidAmount {
				t.Errorf("final current_price = %d, want %d", finalPrice, bidAmount)
			}

			finalCount, err := rdb.HGet(ctx, redisstore.ListingStateKey(listingID), "bid_count").Int()
			if err != nil {
				t.Fatalf("read final bid_count: %v", err)
			}
			if finalCount != 1 {
				t.Errorf("final bid_count = %d, want 1 (a lost update or double-accept would show up here)", finalCount)
			}

			streamLen, err := rdb.XLen(ctx, redisstore.ListingStreamKey(listingID)).Result()
			if err != nil {
				t.Fatalf("read stream length: %v", err)
			}
			if streamLen != 1 {
				t.Errorf("stream length = %d, want exactly 1 (one entry per accepted bid, none for rejections)", streamLen)
			}
		})
	}
}
