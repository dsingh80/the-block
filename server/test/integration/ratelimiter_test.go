//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
)

func TestRateLimiter_Allow(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)

	t.Run("allows up to the limit, rejects the next one, within a window", func(t *testing.T) {
		limiter := redisstore.NewRateLimiter(rdb, 3, time.Minute)

		for i := 1; i <= 3; i++ {
			ok, err := limiter.Allow(ctx, "session-a", "bid")
			if err != nil {
				t.Fatalf("Allow (request %d): %v", i, err)
			}
			if !ok {
				t.Errorf("request %d/3 was rejected, want allowed", i)
			}
		}

		ok, err := limiter.Allow(ctx, "session-a", "bid")
		if err != nil {
			t.Fatalf("Allow (4th request): %v", err)
		}
		if ok {
			t.Error("4th request was allowed, want rejected (limit is 3)")
		}
	})

	t.Run("different sessions and different buckets don't share a counter", func(t *testing.T) {
		limiter := redisstore.NewRateLimiter(rdb, 1, time.Minute)

		if ok, err := limiter.Allow(ctx, "session-b", "bid"); err != nil || !ok {
			t.Fatalf("session-b/bid: ok=%v err=%v, want true/nil", ok, err)
		}
		if ok, err := limiter.Allow(ctx, "session-c", "bid"); err != nil || !ok {
			t.Fatalf("session-c/bid (different session): ok=%v err=%v, want true/nil", ok, err)
		}
		if ok, err := limiter.Allow(ctx, "session-b", "buy-now"); err != nil || !ok {
			t.Fatalf("session-b/buy-now (different bucket): ok=%v err=%v, want true/nil", ok, err)
		}
	})

	t.Run("resets once the window elapses", func(t *testing.T) {
		// Redis's EXPIRE floors at 1 second, same as elsewhere in this suite.
		limiter := redisstore.NewRateLimiter(rdb, 1, time.Second)

		if ok, err := limiter.Allow(ctx, "session-d", "bid"); err != nil || !ok {
			t.Fatalf("1st request: ok=%v err=%v, want true/nil", ok, err)
		}
		if ok, err := limiter.Allow(ctx, "session-d", "bid"); err != nil || ok {
			t.Fatalf("2nd request within the window: ok=%v err=%v, want false/nil", ok, err)
		}

		time.Sleep(1200 * time.Millisecond) // past the 1s window

		if ok, err := limiter.Allow(ctx, "session-d", "bid"); err != nil || !ok {
			t.Fatalf("1st request after the window reset: ok=%v err=%v, want true/nil", ok, err)
		}
	})
}
