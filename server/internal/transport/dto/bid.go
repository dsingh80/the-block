package dto

import (
	"fmt"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Viewer is the wire shape of domain.Viewer -- one session's relationship to a
// listing, computed without a SQL join (guidelines/06-backend-architecture.md,
// "Computing viewer without joins").
type Viewer struct {
	HasBid       bool `json:"has_bid"`
	IsHighBidder bool `json:"is_high_bidder"`
	IsOutbid     bool `json:"is_outbid"`
}

func NewViewer(v domain.Viewer) Viewer {
	return Viewer{HasBid: v.HasBid, IsHighBidder: v.IsHighBidder, IsOutbid: v.IsOutbid}
}

// BidHistoryEntry is one row of a listing's anonymized bid history
// (guidelines/06-backend-architecture.md, "Bid-history anonymization").
type BidHistoryEntry struct {
	Handle     string `json:"handle"`
	Type       string `json:"type"` // "bid" | "buy_now"
	Amount     int64  `json:"amount"`
	BidCount   int    `json:"bid_count"` // bid_count_after -- the running total once this bid landed
	AcceptedAt string `json:"accepted_at"`
	IsViewer   bool   `json:"is_viewer"`
}

// BidHistory is the full GET /v1/listings/{id}/bids response shape.
type BidHistory struct {
	Data []BidHistoryEntry `json:"data"`
}

// NewBidHistory assigns anonymized "Bidder N" handles in order of each
// session's first appearance in bids, which must already be chronological
// (oldest first) -- an anonymized handle is about when a session FIRST
// appeared in real history, not where it falls in whatever order the caller
// happens to iterate (guidelines/06-backend-architecture.md). The response
// itself is returned newest-first, matching how a bid-history feed is read.
//
// bid_count on a listing can legitimately exceed len(bids): pre-seeded
// listings ship a synthetic aggregate count with no bids rows at all --
// real rows only start accumulating once this backend goes live. An empty
// history here is expected, not an error.
func NewBidHistory(bids []domain.Bid, viewerSessionToken string) BidHistory {
	handles := make(map[string]string, len(bids))
	entries := make([]BidHistoryEntry, len(bids))
	for i, b := range bids {
		handle, ok := handles[b.SessionID]
		if !ok {
			handle = fmt.Sprintf("Bidder %d", len(handles)+1)
			handles[b.SessionID] = handle
		}
		entries[i] = BidHistoryEntry{
			Handle:     handle,
			Type:       string(b.Type),
			Amount:     b.Amount,
			BidCount:   b.BidCountAfter,
			AcceptedAt: b.AcceptedAt.UTC().Format(time.RFC3339),
			IsViewer:   viewerSessionToken != "" && b.SessionID == viewerSessionToken,
		}
	}
	reverseBidHistoryEntries(entries)
	return BidHistory{Data: entries}
}

func reverseBidHistoryEntries(e []BidHistoryEntry) {
	for i, j := 0, len(e)-1; i < j; i, j = i+1, j-1 {
		e[i], e[j] = e[j], e[i]
	}
}

// BidAccept is the wire shape of a successful bid or buy-now acceptance
// (guidelines/06-backend-architecture.md, "Idempotency"). Built by the handler
// directly from bidding.Result's fields (the same pattern as ListingsPage),
// not a constructor here -- this package stays domain-only, no usecase import.
type BidAccept struct {
	BidID      string `json:"bid_id"`
	CurrentBid int64  `json:"current_bid"`
	BidCount   int    `json:"bid_count"`
	AcceptedAt string `json:"accepted_at"`
	Viewer     Viewer `json:"viewer"`
}

type BidAcceptResponse struct {
	Data BidAccept `json:"data"`
}

// AcceptedByCaller is the viewer object for whichever session's bid/buy-now was
// just accepted -- always has_bid+is_high_bidder true, is_outbid false, and
// deliberately a constant rather than a fresh Redis lookup: an identical
// retried request must return a byte-identical response to the original
// acceptance (guidelines/06-backend-architecture.md, "Idempotency"), and a live
// lookup on replay could disagree with that if someone else has since outbid
// this session in the meantime. This response describes the accept event
// itself, not a live snapshot of the listing.
var AcceptedByCaller = Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}
