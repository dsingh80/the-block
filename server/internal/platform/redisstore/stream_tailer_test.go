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
