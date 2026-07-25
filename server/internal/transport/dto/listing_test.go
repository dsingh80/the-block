package dto

import (
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

func sampleListingForSummary() domain.Listing {
	reserve := int64(29_000)
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return domain.Listing{
		ID: "abc-123", VIN: "VIN1", Year: 2025, Make: "Mazda", Model: "CX-5",
		AuctionStart: start, AuctionEnd: start.Add(24 * time.Hour),
		StartingBid: 20_500, ReservePrice: &reserve, CurrentPrice: 20_500,
		DamageNotes: []string{}, Images: []string{},
	}
}

func TestNewListingSummary_CurrentBid(t *testing.T) {
	t.Run("bid_count zero -> current_bid is nil, even if CurrentPrice was pre-set to starting_bid", func(t *testing.T) {
		l := sampleListingForSummary()
		l.BidCount = 0

		got := NewListingSummary(l, l.AuctionStart, domain.Viewer{})

		if got.CurrentBid != nil {
			t.Errorf("CurrentBid = %v, want nil when bid_count is 0", *got.CurrentBid)
		}
	})

	t.Run("bid_count > 0 -> current_bid reflects CurrentPrice", func(t *testing.T) {
		l := sampleListingForSummary()
		l.BidCount = 3
		l.CurrentPrice = 22_500

		got := NewListingSummary(l, l.AuctionStart, domain.Viewer{})

		if got.CurrentBid == nil || *got.CurrentBid != 22_500 {
			t.Errorf("CurrentBid = %v, want a pointer to 22500", got.CurrentBid)
		}
	})
}

func TestNewListingSummary_PurchasedAt(t *testing.T) {
	t.Run("nil PurchasedAt -> nil in the wire shape", func(t *testing.T) {
		l := sampleListingForSummary()
		l.PurchasedAt = nil

		got := NewListingSummary(l, l.AuctionStart, domain.Viewer{})

		if got.PurchasedAt != nil {
			t.Errorf("PurchasedAt = %v, want nil", *got.PurchasedAt)
		}
	})

	t.Run("set PurchasedAt -> formatted as RFC3339 in UTC", func(t *testing.T) {
		l := sampleListingForSummary()
		purchased := time.Date(2026, 1, 2, 8, 30, 0, 0, time.FixedZone("EST", -5*60*60))
		l.PurchasedAt = &purchased

		got := NewListingSummary(l, l.AuctionStart, domain.Viewer{})

		want := purchased.UTC().Format(time.RFC3339)
		if got.PurchasedAt == nil || *got.PurchasedAt != want {
			t.Errorf("PurchasedAt = %v, want %q", got.PurchasedAt, want)
		}
	})

	// A completed Buy Now forces status to ended regardless of the clock
	// (domain.Listing.Status) -- worth asserting here too since NewListingSummary
	// is what actually serializes Status(now) onto the wire.
	t.Run("a purchased listing reports status ended even mid-auction", func(t *testing.T) {
		l := sampleListingForSummary()
		purchased := l.AuctionStart.Add(time.Hour)
		l.PurchasedAt = &purchased

		got := NewListingSummary(l, l.AuctionStart.Add(2*time.Hour), domain.Viewer{})

		if got.Status != string(domain.LifecycleEnded) {
			t.Errorf("Status = %q, want %q for a purchased listing", got.Status, domain.LifecycleEnded)
		}
	})
}

func TestNewListingSummary_PassesThroughViewer(t *testing.T) {
	l := sampleListingForSummary()
	viewer := domain.Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true}

	got := NewListingSummary(l, l.AuctionStart, viewer)

	want := Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true}
	if got.Viewer != want {
		t.Errorf("Viewer = %+v, want %+v", got.Viewer, want)
	}
}
