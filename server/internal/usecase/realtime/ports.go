// Package realtime is the fan-out use-case: listing-scoped publish/subscribe for
// WebSocket delivery (guidelines/06-backend-architecture.md). Kept free of any
// specific transport (WS) or backplane (in-memory today, Redis Pub/Sub later).
package realtime

import (
	"context"
	"time"
)

// Event is a listing-scoped notification fanned out to every subscriber
// currently interested in that listing. Recipient-relative fields (e.g. "is
// this you") are deliberately not part of Event -- the identical Event is
// delivered to every subscriber, and each one resolves what's relevant to it
// against its own session (guidelines/06-backend-architecture.md, "computed
// per recipient connection").
type Event struct {
	Type                string // "bid_accepted" | "listing_ended"
	ListingID           string
	BidID               string
	CurrentPrice        int64
	BidCount            int
	HighBidderSessionID string
	AcceptedAt          time.Time
	Reason              string // "time_expired" | "bought_now" -- listing_ended only
}

// Subscriber receives events for whatever listings it's currently subscribed
// to. Implemented by the WS hub's per-connection type (a later commit). Notify
// must not block the publisher for long -- a slow or dead subscriber shouldn't
// stall delivery to everyone else.
type Subscriber interface {
	Notify(event Event)
}

// Broadcaster is the fan-out port. Single-instance today (internal/platform/inmemory);
// a Redis Pub/Sub-backed implementation is a config-flag swap once this ever runs
// as more than one instance (guidelines/06-backend-architecture.md).
type Broadcaster interface {
	Subscribe(subscriber Subscriber, listingIDs ...string)
	Unsubscribe(subscriber Subscriber, listingIDs ...string)
	Publish(ctx context.Context, event Event)
}
