package domain

import "time"

type BidType string

const (
	BidTypeBid    BidType = "bid"
	BidTypeBuyNow BidType = "buy_now"
)

// Bid mirrors a row in the `bids` table -- the durable, append-only per-bid audit
// trail (guidelines/06-backend-architecture.md). Never mutated after insert.
type Bid struct {
	ID             string
	ListingID      string
	SessionID      string
	RequestID      string
	Type           BidType
	Amount         int64
	BidCountAfter  int
	AcceptedAt     time.Time
	SourceStreamID string
}

// Viewer is one session's relationship to a listing. Computed without a SQL join --
// IsHighBidder from a column comparison, HasBid from a Redis set-membership check
// (guidelines/06-backend-architecture.md, "computing viewer without joins"). This is
// structurally what the client's BidOverride used to invent locally from nothing.
type Viewer struct {
	HasBid       bool
	IsHighBidder bool
	IsOutbid     bool
}

// ComputeViewer derives sessionToken's relationship to l from two cheap facts
// instead of a SQL join: IsHighBidder is a plain column comparison (already
// denormalized onto the listing row by the stream-tailer drain), and HasBid is
// a lookup against a set the caller already fetched with one Redis SMEMBERS
// call regardless of how many listings are being rendered
// (guidelines/06-backend-architecture.md, "Computing viewer without joins").
func ComputeViewer(l Listing, sessionToken string, bidListingIDs map[string]struct{}) Viewer {
	_, hasBid := bidListingIDs[l.ID]
	isHighBidder := sessionToken != "" && l.HighBidderSessionID != nil && *l.HighBidderSessionID == sessionToken
	return Viewer{HasBid: hasBid, IsHighBidder: isHighBidder, IsOutbid: hasBid && !isHighBidder}
}

// ReconcileHighBidder corrects a HasBid-but-not-high-bidder Viewer using
// liveHighBidderSessionID -- Redis's own high_bidder_session field, the same
// one place_bid.lua/buy_now.lua write atomically alongside the SADD that
// backs HasBid. It exists because l.HighBidderSessionID (what ComputeViewer
// was given) comes from Postgres, which only reaches consistency once the
// stream tailer's next tick drains a fresh accept into it -- up to
// streamTailerInterval behind Redis (guidelines/06-backend-architecture.md,
// "Draining vs. broadcasting"). HasBid has no such lag (it's answered from
// Redis directly), so in that window a session can already have HasBid=true
// for its own just-accepted bid while Postgres's high_bidder_session_id still
// names the *previous* high bidder -- ComputeViewer alone can't tell that
// apart from a genuine outbid-by-someone-else, since both look identical from
// {HasBid: true, IsHighBidder: false}.
//
// Only ever turns a false-positive IsOutbid back off -- never the reverse -- so
// a caller that skips this entirely (the common case: most viewers have no
// live high bidder to check) is simply left with whatever ComputeViewer
// already had, never made wrong by omission.
func (v Viewer) ReconcileHighBidder(liveHighBidderSessionID, sessionToken string) Viewer {
	if !v.HasBid || v.IsHighBidder || sessionToken == "" || liveHighBidderSessionID != sessionToken {
		return v
	}
	return Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}
}
