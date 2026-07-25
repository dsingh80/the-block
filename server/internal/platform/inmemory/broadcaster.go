// Package inmemory holds the single-instance default implementations of
// cross-cutting platform ports (guidelines/06-backend-architecture.md) --
// today, just the realtime Broadcaster.
package inmemory

import (
	"context"
	"sync"

	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// Broadcaster implements realtime.Broadcaster with an in-process map -- correct
// and sufficient as long as exactly one app instance is running. A
// Redis Pub/Sub-backed implementation takes over this same interface if that
// ever changes (guidelines/06-backend-architecture.md).
type Broadcaster struct {
	mu   sync.RWMutex
	subs map[string]map[realtime.Subscriber]struct{} // listingID -> subscribers
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[string]map[realtime.Subscriber]struct{})}
}

func (b *Broadcaster) Subscribe(sub realtime.Subscriber, listingIDs ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range listingIDs {
		if b.subs[id] == nil {
			b.subs[id] = make(map[realtime.Subscriber]struct{})
		}
		b.subs[id][sub] = struct{}{}
	}
}

func (b *Broadcaster) Unsubscribe(sub realtime.Subscriber, listingIDs ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range listingIDs {
		delete(b.subs[id], sub)
		if len(b.subs[id]) == 0 {
			delete(b.subs, id)
		}
	}
}

func (b *Broadcaster) Publish(_ context.Context, event realtime.Event) {
	// Copy the subscriber set under the lock, then Notify outside it -- a slow
	// or dead subscriber's Notify must not block Subscribe/Unsubscribe/other
	// Publish calls while it's running.
	b.mu.RLock()
	recipients := make([]realtime.Subscriber, 0, len(b.subs[event.ListingID]))
	for sub := range b.subs[event.ListingID] {
		recipients = append(recipients, sub)
	}
	b.mu.RUnlock()

	for _, sub := range recipients {
		sub.Notify(event)
	}
}
