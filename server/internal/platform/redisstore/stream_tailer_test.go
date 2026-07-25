package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Run's shutdown contract matters on its own, independent of anything it
// actually drains: cmd/api/main.go runs it for the life of the process and
// relies on ctx cancellation being a reliable way to stop it, not something
// that depends on how long the tick interval happens to be. An interval far
// longer than this test can run guarantees ctx.Done() is what wins the
// select, never a coincidental tick -- so this needs no real Redis or
// Postgres at all: Tick() (the part that would touch them) is never reached.
func TestStreamTailer_Run_ReturnsPromptlyOnContextCancellation(t *testing.T) {
	tailer := NewStreamTailer(nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- tailer.Run(ctx, time.Hour) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return within 1s of context cancellation")
	}
}

// parseFieldInt64 is pure -- the real malformed-stream-entry scenario it
// guards against (a bad field actually reaching processEntry through Tick) is
// covered by test/integration's TestStreamTailer_Tick_MalformedStreamEntryIsLoggedAndSkipped,
// which needs a real Redis; this only covers the parsing helper in isolation.
func TestParseFieldInt64(t *testing.T) {
	t.Run("a numeric string parses", func(t *testing.T) {
		got, err := parseFieldInt64("21500")
		if err != nil || got != 21_500 {
			t.Errorf("parseFieldInt64(\"21500\") = (%v, %v), want (21500, nil)", got, err)
		}
	})

	t.Run("a non-string value (Redis field values are always strings) is rejected", func(t *testing.T) {
		if _, err := parseFieldInt64(21_500); err == nil {
			t.Error("expected an error for a non-string field value, got nil")
		}
	})

	t.Run("a non-numeric string is rejected", func(t *testing.T) {
		if _, err := parseFieldInt64("not-a-number"); err == nil {
			t.Error("expected an error for a non-numeric string, got nil")
		}
	})

	t.Run("nil is rejected", func(t *testing.T) {
		if _, err := parseFieldInt64(nil); err == nil {
			t.Error("expected an error for a nil field value, got nil")
		}
	})
}
