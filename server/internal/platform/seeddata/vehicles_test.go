package seeddata

import (
	"testing"
	"time"
)

func TestLoadVehicles(t *testing.T) {
	listings, err := LoadVehicles("testdata/sample.json")
	if err != nil {
		t.Fatalf("LoadVehicles: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("got %d listings, want 2", len(listings))
	}

	t.Run("auction_start is parsed as UTC from a zone-less string", func(t *testing.T) {
		want := time.Date(2026, 7, 28, 19, 0, 0, 0, time.UTC)
		if !listings[0].AuctionStart.Equal(want) {
			t.Errorf("AuctionStart = %v, want %v", listings[0].AuctionStart, want)
		}
		if listings[0].AuctionStart.Location() != time.UTC {
			t.Errorf("AuctionStart location = %v, want UTC", listings[0].AuctionStart.Location())
		}
	})

	t.Run("every listing gets the default 24h duration", func(t *testing.T) {
		for _, l := range listings {
			if l.AuctionDuration != DefaultAuctionDuration {
				t.Errorf("%s: AuctionDuration = %v, want %v", l.ID, l.AuctionDuration, DefaultAuctionDuration)
			}
		}
	})

	t.Run("current_bid set -> CurrentPrice uses it, not starting_bid", func(t *testing.T) {
		l := listings[0]
		if l.CurrentPrice != 21_000 {
			t.Errorf("CurrentPrice = %d, want 21000 (current_bid)", l.CurrentPrice)
		}
		if l.BidCount != 11 {
			t.Errorf("BidCount = %d, want 11", l.BidCount)
		}
	})

	t.Run("current_bid null -> CurrentPrice falls back to starting_bid", func(t *testing.T) {
		l := listings[1]
		if l.CurrentPrice != l.StartingBid {
			t.Errorf("CurrentPrice = %d, want %d (starting_bid, since current_bid was null)", l.CurrentPrice, l.StartingBid)
		}
	})

	t.Run("nullable reserve_price/buy_now_price round-trip both ways", func(t *testing.T) {
		if listings[0].ReservePrice == nil || *listings[0].ReservePrice != 29_000 {
			t.Errorf("listings[0].ReservePrice = %v, want 29000", listings[0].ReservePrice)
		}
		if listings[0].BuyNowPrice != nil {
			t.Errorf("listings[0].BuyNowPrice = %v, want nil", listings[0].BuyNowPrice)
		}
		if listings[1].ReservePrice != nil {
			t.Errorf("listings[1].ReservePrice = %v, want nil", listings[1].ReservePrice)
		}
		if listings[1].BuyNowPrice == nil || *listings[1].BuyNowPrice != 15_000 {
			t.Errorf("listings[1].BuyNowPrice = %v, want 15000", listings[1].BuyNowPrice)
		}
	})

	t.Run("an empty damage_notes array stays a non-nil empty slice", func(t *testing.T) {
		if listings[0].DamageNotes == nil {
			t.Error("DamageNotes is nil, want a non-nil empty slice (must not violate the NOT NULL column)")
		}
		if len(listings[0].DamageNotes) != 0 {
			t.Errorf("DamageNotes = %v, want empty", listings[0].DamageNotes)
		}
		if len(listings[1].DamageNotes) != 2 {
			t.Errorf("DamageNotes = %v, want 2 entries", listings[1].DamageNotes)
		}
	})
}

func TestLoadVehicles_MissingFile(t *testing.T) {
	if _, err := LoadVehicles("testdata/does-not-exist.json"); err == nil {
		t.Error("expected an error loading a nonexistent file, got nil")
	}
}
