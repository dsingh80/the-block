package dto

import (
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

func TestNewBidHistory_EmptyBidsReturnsEmptyList(t *testing.T) {
	got := NewBidHistory(nil, "session-a")
	if got.Data == nil {
		t.Fatal("Data = nil, want a non-nil empty slice (must marshal to [], not null)")
	}
	if len(got.Data) != 0 {
		t.Errorf("Data = %+v, want empty", got.Data)
	}
}

func TestNewBidHistory_AssignsHandlesByFirstAppearanceAndDisplaysNewestFirst(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	bids := []domain.Bid{
		// Chronological (oldest first), as audit.Reader.ListForListing guarantees.
		{SessionID: "session-a", Type: domain.BidTypeBid, Amount: 21_000, BidCountAfter: 1, AcceptedAt: t0},
		{SessionID: "session-b", Type: domain.BidTypeBid, Amount: 21_500, BidCountAfter: 2, AcceptedAt: t0.Add(time.Minute)},
		{SessionID: "session-a", Type: domain.BidTypeBid, Amount: 22_000, BidCountAfter: 3, AcceptedAt: t0.Add(2 * time.Minute)},
	}

	got := NewBidHistory(bids, "session-b")

	if len(got.Data) != 3 {
		t.Fatalf("len(Data) = %d, want 3", len(got.Data))
	}
	// Newest first: session-a's second bid, then session-b's, then session-a's first.
	// session-a is "Bidder 1" (first ever seen), session-b is "Bidder 2" -- in BOTH
	// of its appearances, session-a keeps the handle it earned on first appearance,
	// not a new one on its second bid.
	want := []struct {
		handle   string
		amount   int64
		isViewer bool
	}{
		{"Bidder 1", 22_000, false},
		{"Bidder 2", 21_500, true},
		{"Bidder 1", 21_000, false},
	}
	for i, w := range want {
		e := got.Data[i]
		if e.Handle != w.handle || e.Amount != w.amount || e.IsViewer != w.isViewer {
			t.Errorf("Data[%d] = {handle:%q amount:%d is_viewer:%v}, want {%q %d %v}",
				i, e.Handle, e.Amount, e.IsViewer, w.handle, w.amount, w.isViewer)
		}
	}
}
