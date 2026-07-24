//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
)

func TestSessionStore_Touch(t *testing.T) {
	ctx := context.Background()
	rdb := startRedis(t)
	store := redisstore.NewSessionStore(rdb, time.Minute)

	t.Run("an empty token creates a brand-new session", func(t *testing.T) {
		session, isNew, err := store.Touch(ctx, "")
		if err != nil {
			t.Fatalf("Touch: %v", err)
		}
		if !isNew {
			t.Error("isNew = false, want true for an empty token")
		}
		if session.Token == "" {
			t.Error("expected a generated, non-empty token")
		}
		if session.CreatedAt.IsZero() || session.LastSeenAt.IsZero() {
			t.Errorf("expected non-zero timestamps, got %+v", session)
		}
	})

	t.Run("an unknown token is ignored -- a fresh token is generated, not reused", func(t *testing.T) {
		session, isNew, err := store.Touch(ctx, "some-forged-or-stale-token")
		if err != nil {
			t.Fatalf("Touch: %v", err)
		}
		if !isNew {
			t.Error("isNew = false, want true for an unknown token")
		}
		if session.Token == "some-forged-or-stale-token" {
			t.Error("expected a freshly generated token, not the caller-supplied one")
		}
	})

	t.Run("touching an existing valid token refreshes it instead of creating a new one", func(t *testing.T) {
		original, _, err := store.Touch(ctx, "")
		if err != nil {
			t.Fatalf("Touch (create): %v", err)
		}

		time.Sleep(10 * time.Millisecond) // ensure a measurable last_seen_at delta

		refreshed, isNew, err := store.Touch(ctx, original.Token)
		if err != nil {
			t.Fatalf("Touch (refresh): %v", err)
		}
		if isNew {
			t.Error("isNew = true, want false when refreshing an existing session")
		}
		if refreshed.Token != original.Token {
			t.Errorf("Token = %s, want the same token (%s) to be preserved on refresh", refreshed.Token, original.Token)
		}
		// Millisecond granularity, not .Equal(): created_at is stored in Redis as
		// Unix milliseconds, so the round trip through Redis is expected to drop
		// the sub-millisecond remainder Go's own time.Now() carries -- that's a
		// deliberate precision choice for session timestamps, not a bug to
		// compare away with an over-precise assertion.
		if refreshed.CreatedAt.UnixMilli() != original.CreatedAt.UnixMilli() {
			t.Errorf("CreatedAt = %v, want it unchanged from the original %v", refreshed.CreatedAt, original.CreatedAt)
		}
		if !refreshed.LastSeenAt.After(original.LastSeenAt) {
			t.Errorf("LastSeenAt (%v) should have advanced past the original (%v)", refreshed.LastSeenAt, original.LastSeenAt)
		}
	})

	t.Run("an expired session is treated as absent, not resurrected", func(t *testing.T) {
		// go-redis's Expire rounds sub-second durations up to Redis's minimum of
		// 1 second (confirmed by hitting its own truncation warning with a
		// shorter value first) -- 1s is as short as this can be made, so the
		// test just accepts the ~1s wait rather than fighting that floor.
		shortLived := redisstore.NewSessionStore(rdb, time.Second)

		original, _, err := shortLived.Touch(ctx, "")
		if err != nil {
			t.Fatalf("Touch (create): %v", err)
		}

		time.Sleep(1200 * time.Millisecond) // past the 1s TTL

		after, isNew, err := shortLived.Touch(ctx, original.Token)
		if err != nil {
			t.Fatalf("Touch (post-expiry): %v", err)
		}
		if !isNew {
			t.Error("isNew = false, want true -- the original session should have expired out of Redis")
		}
		if after.Token == original.Token {
			t.Error("expected a new token after expiry, got the same one back")
		}
	})
}
