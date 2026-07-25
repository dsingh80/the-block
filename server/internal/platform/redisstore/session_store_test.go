package redisstore

import (
	"strconv"
	"testing"
	"time"
)

// parseUnixMillisString and generateSessionToken are pure -- the actual Redis
// read/write behavior around them (Touch's create/refresh/expiry paths) is
// covered by test/integration's TestSessionStore_Touch, which needs a real
// Redis; these only cover the two helpers in isolation.

func TestParseUnixMillisString(t *testing.T) {
	t.Run("a valid millisecond timestamp round-trips", func(t *testing.T) {
		want := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		got, err := parseUnixMillisString(strconv.FormatInt(want.UnixMilli(), 10))
		if err != nil {
			t.Fatalf("parseUnixMillisString: %v", err)
		}
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("a non-numeric value is rejected", func(t *testing.T) {
		if _, err := parseUnixMillisString("not-a-number"); err == nil {
			t.Error("expected an error for a non-numeric value, got nil")
		}
	})

	t.Run("an empty string is rejected", func(t *testing.T) {
		if _, err := parseUnixMillisString(""); err == nil {
			t.Error("expected an error for an empty string, got nil")
		}
	})
}

func TestGenerateSessionToken(t *testing.T) {
	a, err := generateSessionToken()
	if err != nil {
		t.Fatalf("generateSessionToken: %v", err)
	}
	if a == "" {
		t.Fatal("got an empty token")
	}

	b, err := generateSessionToken()
	if err != nil {
		t.Fatalf("generateSessionToken: %v", err)
	}
	if a == b {
		t.Error("two calls returned the same token, want independently random values")
	}
}
