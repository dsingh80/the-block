package redisstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a fixed-window counter per (session, bucket) -- e.g. bid
// attempts per session within a window (guidelines/06-backend-architecture.md).
// Mitigates abuse without needing real auth first.
type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

// Allow increments the counter for (sessionToken, bucket) and reports whether
// this request is within the limit for the current window. The counter starts
// its expiry on the first hit of a new window and resets (via that expiry)
// once the window elapses -- there's no separate reset step to forget to call.
func (r *RateLimiter) Allow(ctx context.Context, sessionToken, bucket string) (bool, error) {
	key := RateLimitKey(sessionToken, bucket)

	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redisstore: incr rate limit %s: %w", key, err)
	}

	if count == 1 {
		if err := r.rdb.Expire(ctx, key, r.window).Err(); err != nil {
			return false, fmt.Errorf("redisstore: set rate limit window for %s: %w", key, err)
		}
	}

	return count <= int64(r.limit), nil
}
