package inmemory

import (
	"context"
	"sync"
	"testing"

	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// fakeSubscriber records every event it's notified of, safe for concurrent use.
type fakeSubscriber struct {
	mu     sync.Mutex
	events []realtime.Event
}

func (f *fakeSubscriber) Notify(e realtime.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeSubscriber) received() []realtime.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]realtime.Event, len(f.events))
	copy(out, f.events)
	return out
}

func TestBroadcaster_PublishOnlyReachesSubscribedListeners(t *testing.T) {
	ctx := context.Background()
	b := NewBroadcaster()

	subA, subB := &fakeSubscriber{}, &fakeSubscriber{}
	b.Subscribe(subA, "listing-1")
	b.Subscribe(subB, "listing-2")

	b.Publish(ctx, realtime.Event{Type: "bid_accepted", ListingID: "listing-1", BidID: "bid-1"})

	if got := subA.received(); len(got) != 1 || got[0].BidID != "bid-1" {
		t.Errorf("subA.received() = %+v, want exactly the listing-1 event", got)
	}
	if got := subB.received(); len(got) != 0 {
		t.Errorf("subB.received() = %+v, want none (not subscribed to listing-1)", got)
	}

	b.Publish(ctx, realtime.Event{Type: "bid_accepted", ListingID: "listing-2", BidID: "bid-2"})

	if got := subA.received(); len(got) != 1 {
		t.Errorf("subA.received() = %+v, want still just the one listing-1 event", got)
	}
	if got := subB.received(); len(got) != 1 || got[0].BidID != "bid-2" {
		t.Errorf("subB.received() = %+v, want exactly the listing-2 event", got)
	}
}

func TestBroadcaster_SubscribeToMultipleListings(t *testing.T) {
	ctx := context.Background()
	b := NewBroadcaster()

	sub := &fakeSubscriber{}
	b.Subscribe(sub, "listing-1", "listing-2")

	b.Publish(ctx, realtime.Event{ListingID: "listing-1", BidID: "a"})
	b.Publish(ctx, realtime.Event{ListingID: "listing-2", BidID: "b"})
	b.Publish(ctx, realtime.Event{ListingID: "listing-3", BidID: "c"}) // not subscribed

	got := sub.received()
	if len(got) != 2 {
		t.Fatalf("received %d events, want 2 (listing-3 shouldn't reach a subscriber that never subscribed to it)", len(got))
	}
	if got[0].BidID != "a" || got[1].BidID != "b" {
		t.Errorf("received = %+v, want [a, b] in order", got)
	}
}

func TestBroadcaster_Unsubscribe(t *testing.T) {
	ctx := context.Background()
	b := NewBroadcaster()

	sub := &fakeSubscriber{}
	b.Subscribe(sub, "listing-1")
	b.Publish(ctx, realtime.Event{ListingID: "listing-1", BidID: "before-unsub"})

	b.Unsubscribe(sub, "listing-1")
	b.Publish(ctx, realtime.Event{ListingID: "listing-1", BidID: "after-unsub"})

	got := sub.received()
	if len(got) != 1 || got[0].BidID != "before-unsub" {
		t.Errorf("received = %+v, want only the event published before Unsubscribe", got)
	}
}

func TestBroadcaster_MultipleSubscribersToOneListing(t *testing.T) {
	ctx := context.Background()
	b := NewBroadcaster()

	subA, subB := &fakeSubscriber{}, &fakeSubscriber{}
	b.Subscribe(subA, "listing-1")
	b.Subscribe(subB, "listing-1")

	b.Publish(ctx, realtime.Event{ListingID: "listing-1", BidID: "bid-1"})

	if len(subA.received()) != 1 {
		t.Error("subA did not receive the event both subscribers were subscribed to")
	}
	if len(subB.received()) != 1 {
		t.Error("subB did not receive the event both subscribers were subscribed to")
	}
}
