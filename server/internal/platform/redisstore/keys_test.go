package redisstore

import (
	"strings"
	"testing"
)

func TestListingKeys_ShareAHashTag(t *testing.T) {
	state := ListingStateKey("abc-123")
	stream := ListingStreamKey("abc-123")

	if state != "{listing:abc-123}:state" {
		t.Errorf("ListingStateKey = %q", state)
	}
	if stream != "{listing:abc-123}:stream" {
		t.Errorf("ListingStreamKey = %q", stream)
	}

	// Both keys must share the exact same {...} hash-tag substring, or a future
	// Redis Cluster move could route them to different slots and break the Lua
	// script's ability to touch both atomically.
	tagOf := func(key string) string {
		start, end := strings.Index(key, "{"), strings.Index(key, "}")
		return key[start : end+1]
	}
	if tagOf(state) != tagOf(stream) {
		t.Errorf("hash tags differ: %q vs %q", tagOf(state), tagOf(stream))
	}
}

func TestOtherKeyBuilders(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"SessionKey", SessionKey("tok1"), "session:tok1"},
		{"SessionBidsKey", SessionBidsKey("tok1"), "session:tok1:bids"},
		{"IdempotencyKey", IdempotencyKey("tok1", "listing1", 21500), "idem:tok1:listing1:21500"},
		{"IdempotencyBuyNowKey", IdempotencyBuyNowKey("tok1", "listing1"), "idem_buy:tok1:listing1"},
		{"RateLimitKey", RateLimitKey("tok1", "bid"), "ratelimit:tok1:bid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestIdempotencyKey_DifferentAmountsGiveDifferentKeys(t *testing.T) {
	a := IdempotencyKey("tok1", "listing1", 21500)
	b := IdempotencyKey("tok1", "listing1", 21600)
	if a == b {
		t.Errorf("expected different amounts to produce different idempotency keys, both were %q", a)
	}
}
