// Package ws is the WebSocket transport: a per-connection Hub that upgrades
// requests and wires connections to realtime.Broadcaster (guidelines/06-backend-architecture.md,
// "WebSocket protocol"). Bidding itself stays a REST action; this package is a
// pure push channel for observing other sessions' activity.
package ws

// clientMessage is the client -> server shape: {"type":"subscribe"|"unsubscribe","listing_ids":[...]}.
type clientMessage struct {
	Type       string   `json:"type"`
	ListingIDs []string `json:"listing_ids"`
}

// ackMessage confirms a subscribe/unsubscribe with the connection's full
// current subscription set, not just what the triggering message asked for --
// a client that sends overlapping subscribe calls always sees ground truth.
type ackMessage struct {
	Type       string   `json:"type"`
	Subscribed []string `json:"subscribed"`
}

// bidAcceptedMessage is a realtime.Event{Type:"bid_accepted"} translated for
// one specific recipient connection -- high_bidder_is_you is the one field
// that differs per recipient (guidelines/06-backend-architecture.md, "computed
// per recipient connection").
type bidAcceptedMessage struct {
	Type            string `json:"type"`
	ListingID       string `json:"listing_id"`
	BidID           string `json:"bid_id"`
	CurrentBid      int64  `json:"current_bid"`
	BidCount        int    `json:"bid_count"`
	HighBidderIsYou bool   `json:"high_bidder_is_you"`
	AcceptedAt      string `json:"accepted_at"`
}

type listingEndedMessage struct {
	Type       string `json:"type"`
	ListingID  string `json:"listing_id"`
	Reason     string `json:"reason"` // "time_expired" | "bought_now"
	FinalPrice int64  `json:"final_price"`
}

type errorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
