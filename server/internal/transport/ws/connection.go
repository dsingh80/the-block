package ws

import (
	"sync"
	"time"

	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// MaxSubscriptionsPerConnection caps how many listing ids one connection can
// subscribe to at once (guidelines/06-backend-architecture.md, "WebSocket protocol").
const MaxSubscriptionsPerConnection = 100

// wireWriter is satisfied by *websocket.Conn's WriteJSON -- a narrow seam so
// Connection's own logic (subscription bookkeeping, translating a
// realtime.Event into this recipient's own wire message) is unit-testable
// without a real network socket (guidelines/06-backend-architecture.md).
type wireWriter interface {
	WriteJSON(v any) error
}

// Connection is one client's WS session. It implements realtime.Subscriber
// directly, so a Broadcaster can Notify it without any adapter, and it tracks
// its own subscribed listing ids plus the session token needed to compute
// high_bidder_is_you per recipient (guidelines/06-backend-architecture.md,
// "computed per recipient connection" -- the Hub resolves this connection's
// session from the same cookie middleware at upgrade time).
type Connection struct {
	writer       wireWriter
	sessionToken string

	mu            sync.Mutex
	subscriptions map[string]struct{}
}

func NewConnection(writer wireWriter, sessionToken string) *Connection {
	return &Connection{writer: writer, sessionToken: sessionToken, subscriptions: make(map[string]struct{})}
}

// Subscribe adds listingIDs to this connection's set, capped at
// MaxSubscriptionsPerConnection. Returns the full set actually subscribed
// (already-subscribed ids are reported again, not double-counted against the
// cap) and whether the cap was hit before every requested id could be added.
func (c *Connection) Subscribe(listingIDs []string) (subscribed []string, capped bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range listingIDs {
		if _, ok := c.subscriptions[id]; !ok {
			if len(c.subscriptions) >= MaxSubscriptionsPerConnection {
				capped = true
				continue
			}
			c.subscriptions[id] = struct{}{}
		}
	}
	return c.subscribedLocked(), capped
}

func (c *Connection) Unsubscribe(listingIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range listingIDs {
		delete(c.subscriptions, id)
	}
}

// SubscribedListingIDs returns a snapshot of the current subscription set --
// what the Hub hands the Broadcaster.
func (c *Connection) SubscribedListingIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subscribedLocked()
}

func (c *Connection) subscribedLocked() []string {
	ids := make([]string, 0, len(c.subscriptions))
	for id := range c.subscriptions {
		ids = append(ids, id)
	}
	return ids
}

func (c *Connection) SendAck(subscribed []string) error {
	return c.writer.WriteJSON(ackMessage{Type: "ack", Subscribed: subscribed})
}

func (c *Connection) SendError(code, message string) error {
	return c.writer.WriteJSON(errorMessage{Type: "error", Code: code, Message: message})
}

// Notify implements realtime.Subscriber. A write failure here (a dead or slow
// connection) is deliberately swallowed, not returned or logged through this
// path: Notify must not block or fail the publisher for one bad subscriber
// (internal/usecase/realtime's own contract) -- the read loop noticing this
// connection is actually gone (a failed Read, or a missed pong) is what
// actually cleans it up, in Hub.serve.
func (c *Connection) Notify(event realtime.Event) {
	switch event.Type {
	case "bid_accepted":
		_ = c.writer.WriteJSON(bidAcceptedMessage{
			Type:            "bid_accepted",
			ListingID:       event.ListingID,
			BidID:           event.BidID,
			CurrentBid:      event.CurrentPrice,
			BidCount:        event.BidCount,
			HighBidderIsYou: c.sessionToken != "" && event.HighBidderSessionID == c.sessionToken,
			AcceptedAt:      event.AcceptedAt.UTC().Format(time.RFC3339),
		})
	case "listing_ended":
		_ = c.writer.WriteJSON(listingEndedMessage{
			Type:       "listing_ended",
			ListingID:  event.ListingID,
			Reason:     event.Reason,
			FinalPrice: event.CurrentPrice,
		})
	}
}
