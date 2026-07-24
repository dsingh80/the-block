package ws

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// fakeWireWriter is a spy for wireWriter -- lets Connection's own logic be
// tested without a real network socket.
type fakeWireWriter struct {
	sent []any
	err  error
}

func (f *fakeWireWriter) WriteJSON(v any) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, v)
	return nil
}

func TestConnection_Subscribe_DedupesAndReturnsFullSet(t *testing.T) {
	c := NewConnection(&fakeWireWriter{}, "session-a")

	subscribed, capped := c.Subscribe([]string{"listing-1", "listing-2"})
	if capped {
		t.Error("capped = true, want false")
	}
	if len(subscribed) != 2 {
		t.Errorf("subscribed = %v, want 2 ids", subscribed)
	}

	// Re-subscribing to an already-subscribed id plus one genuinely new one.
	subscribed, capped = c.Subscribe([]string{"listing-1", "listing-3"})
	if capped {
		t.Error("capped = true, want false")
	}
	if len(subscribed) != 3 {
		t.Errorf("subscribed = %v, want 3 ids total (the repeat isn't double-counted)", subscribed)
	}
}

func TestConnection_Subscribe_EnforcesCap(t *testing.T) {
	c := NewConnection(&fakeWireWriter{}, "session-a")

	ids := make([]string, MaxSubscriptionsPerConnection+10)
	for i := range ids {
		ids[i] = fmt.Sprintf("listing-%d", i)
	}

	subscribed, capped := c.Subscribe(ids)
	if !capped {
		t.Error("capped = false, want true when requesting more than the cap in one call")
	}
	if len(subscribed) != MaxSubscriptionsPerConnection {
		t.Errorf("subscribed %d ids, want exactly the cap (%d)", len(subscribed), MaxSubscriptionsPerConnection)
	}
}

func TestConnection_Unsubscribe(t *testing.T) {
	c := NewConnection(&fakeWireWriter{}, "session-a")
	c.Subscribe([]string{"listing-1", "listing-2"})
	c.Unsubscribe([]string{"listing-1"})

	got := c.SubscribedListingIDs()
	if len(got) != 1 || got[0] != "listing-2" {
		t.Errorf("SubscribedListingIDs() = %v, want [listing-2]", got)
	}
}

func TestConnection_Notify_BidAcceptedComputesHighBidderIsYouPerRecipient(t *testing.T) {
	tests := []struct {
		name                string
		sessionToken        string
		highBidderSessionID string
		want                bool
	}{
		{"this connection's session is the high bidder", "session-a", "session-a", true},
		{"a different session is the high bidder", "session-a", "session-b", false},
		{"no session resolved for this connection at all", "", "session-b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &fakeWireWriter{}
			c := NewConnection(w, tt.sessionToken)
			c.Notify(realtime.Event{
				Type: "bid_accepted", ListingID: "listing-1", BidID: "bid-1",
				CurrentPrice: 21_500, BidCount: 5, HighBidderSessionID: tt.highBidderSessionID,
				AcceptedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			})

			if len(w.sent) != 1 {
				t.Fatalf("sent %d messages, want 1", len(w.sent))
			}
			msg, ok := w.sent[0].(bidAcceptedMessage)
			if !ok {
				t.Fatalf("sent %T, want bidAcceptedMessage", w.sent[0])
			}
			if msg.HighBidderIsYou != tt.want {
				t.Errorf("HighBidderIsYou = %v, want %v", msg.HighBidderIsYou, tt.want)
			}
			if msg.CurrentBid != 21_500 || msg.BidCount != 5 || msg.ListingID != "listing-1" {
				t.Errorf("message = %+v, want the event's own fields carried through unchanged", msg)
			}
		})
	}
}

func TestConnection_Notify_ListingEnded(t *testing.T) {
	w := &fakeWireWriter{}
	c := NewConnection(w, "session-a")
	c.Notify(realtime.Event{Type: "listing_ended", ListingID: "listing-1", Reason: "bought_now", CurrentPrice: 32_000})

	if len(w.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(w.sent))
	}
	msg, ok := w.sent[0].(listingEndedMessage)
	if !ok {
		t.Fatalf("sent %T, want listingEndedMessage", w.sent[0])
	}
	if msg.Reason != "bought_now" || msg.FinalPrice != 32_000 {
		t.Errorf("message = %+v, want reason=bought_now final_price=32000", msg)
	}
}

// A dead/slow connection's write failure must never propagate out of Notify --
// realtime.Subscriber's contract is that Notify can't block or fail the
// publisher for one bad subscriber (internal/usecase/realtime).
func TestConnection_Notify_WriteFailureDoesNotPanicOrReturnAnything(t *testing.T) {
	c := NewConnection(&fakeWireWriter{err: errors.New("connection closed")}, "session-a")
	c.Notify(realtime.Event{Type: "bid_accepted", ListingID: "listing-1"})
}
