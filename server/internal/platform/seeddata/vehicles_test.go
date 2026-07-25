package seeddata

import (
	"strings"
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

func TestLoadVehicles_MalformedJSON(t *testing.T) {
	_, err := LoadVehicles("testdata/malformed.json")
	if err == nil {
		t.Fatal("expected an error loading malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %q, want it to mention parsing", err.Error())
	}
}

// TestLoadVehicles_VehicleConversionErrorIsWrappedWithItsID covers LoadVehicles'
// own wrapping of a per-vehicle toListing failure -- distinct from
// TestRawVehicleToListing_InvalidAuctionStart below, which covers toListing's
// own error return in isolation.
func TestLoadVehicles_VehicleConversionErrorIsWrappedWithItsID(t *testing.T) {
	_, err := LoadVehicles("testdata/bad_auction_start.json")
	if err == nil {
		t.Fatal("expected an error for a vehicle with an unparseable auction_start, got nil")
	}
	if !strings.Contains(err.Error(), "bad-vehicle-1") {
		t.Errorf("error = %q, want it to identify the offending vehicle id (bad-vehicle-1)", err.Error())
	}
}

func TestRawVehicleToListing_InvalidAuctionStart(t *testing.T) {
	v := rawVehicle{ID: "abc", AuctionStart: "not-a-valid-date"}
	if _, err := v.toListing(); err == nil {
		t.Error("expected an error for an unparseable auction_start, got nil")
	}
}

func TestNonNilStrings(t *testing.T) {
	t.Run("nil becomes a non-nil empty slice", func(t *testing.T) {
		got := nonNilStrings(nil)
		if got == nil {
			t.Fatal("got nil, want a non-nil empty slice")
		}
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("a non-nil slice, including an already-empty one, passes through unchanged", func(t *testing.T) {
		in := []string{"a", "b"}
		if got := nonNilStrings(in); len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("nonNilStrings(%v) = %v, want it unchanged", in, got)
		}
		empty := []string{}
		if got := nonNilStrings(empty); got == nil || len(got) != 0 {
			t.Errorf("nonNilStrings(%v) = %v, want an unchanged non-nil empty slice", empty, got)
		}
	})
}
