// Package redisstore holds the Redis hot path: atomic bid accept, sessions, the
// per-listing audit/fan-out stream, and rate limiting (guidelines/06-backend-architecture.md).
package redisstore

import "fmt"

// Key builders are centralized here so no key format is ever hand-typed at a call
// site (guidelines/06-backend-architecture.md, "Go conventions"). Hash-tag braces
// on a listing's own keys keep them co-located and scriptable together if this
// ever moves to Redis Cluster -- costs nothing single-instance.

func ListingStateKey(listingID string) string {
	return fmt.Sprintf("{listing:%s}:state", listingID)
}

func ListingStreamKey(listingID string) string {
	return fmt.Sprintf("{listing:%s}:stream", listingID)
}

func SessionKey(token string) string {
	return fmt.Sprintf("session:%s", token)
}

// SessionBidsKey is the set a session's accepted bids are recorded into (SADD'd
// by the place-bid/buy-now Lua scripts), backing the join-free `viewer.has_bid`
// computation (guidelines/06-backend-architecture.md).
func SessionBidsKey(token string) string {
	return fmt.Sprintf("session:%s:bids", token)
}

// IdempotencyKey is derived from the request itself, never client-supplied
// (guidelines/06-backend-architecture.md, "Idempotency") -- the same
// (session, listing, amount) triple always maps to the same key.
func IdempotencyKey(sessionToken, listingID string, amount int64) string {
	return fmt.Sprintf("idem:%s:%s:%d", sessionToken, listingID, amount)
}

// IdempotencyBuyNowKey has no amount component -- a listing has exactly one
// buy-now price, so (session, listing) alone is already collision-free.
func IdempotencyBuyNowKey(sessionToken, listingID string) string {
	return fmt.Sprintf("idem_buy:%s:%s", sessionToken, listingID)
}

func RateLimitKey(sessionToken, bucket string) string {
	return fmt.Sprintf("ratelimit:%s:%s", sessionToken, bucket)
}
