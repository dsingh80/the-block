package pgstore

import (
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// These are the pure helpers behind ListPage's query building -- no Postgres
// needed, unlike the rest of this package (see test/integration/listing_page_test.go
// for the query-behavior tests that do need a real database).

func TestParseColumnValue(t *testing.T) {
	t.Run("current_price parses as an integer", func(t *testing.T) {
		got, err := parseColumnValue("current_price", "21500")
		if err != nil || got != int64(21_500) {
			t.Errorf("parseColumnValue(current_price, 21500) = (%v, %v), want (21500, nil)", got, err)
		}
	})

	t.Run("current_price rejects a non-numeric value", func(t *testing.T) {
		if _, err := parseColumnValue("current_price", "not-a-number"); err == nil {
			t.Error("expected an error for a non-numeric current_price cursor value")
		}
	})

	t.Run("year parses as an integer", func(t *testing.T) {
		got, err := parseColumnValue("year", "2025")
		if err != nil || got != 2025 {
			t.Errorf("parseColumnValue(year, 2025) = (%v, %v), want (2025, nil)", got, err)
		}
	})

	t.Run("year rejects a non-numeric value", func(t *testing.T) {
		if _, err := parseColumnValue("year", "not-a-year"); err == nil {
			t.Error("expected an error for a non-numeric year cursor value")
		}
	})

	t.Run("auction_end parses as RFC3339Nano", func(t *testing.T) {
		want := time.Date(2026, 1, 1, 12, 0, 0, 123_000_000, time.UTC)
		got, err := parseColumnValue("auction_end", want.Format(time.RFC3339Nano))
		if err != nil {
			t.Fatalf("parseColumnValue(auction_end, ...) error = %v", err)
		}
		gotTime, ok := got.(time.Time)
		if !ok || !gotTime.Equal(want) {
			t.Errorf("parseColumnValue(auction_end, ...) = %v, want %v", got, want)
		}
	})

	t.Run("auction_start parses as RFC3339Nano", func(t *testing.T) {
		want := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		got, err := parseColumnValue("auction_start", want.Format(time.RFC3339Nano))
		if err != nil {
			t.Fatalf("parseColumnValue(auction_start, ...) error = %v", err)
		}
		gotTime, ok := got.(time.Time)
		if !ok || !gotTime.Equal(want) {
			t.Errorf("parseColumnValue(auction_start, ...) = %v, want %v", got, want)
		}
	})

	t.Run("auction_end rejects a malformed timestamp", func(t *testing.T) {
		if _, err := parseColumnValue("auction_end", "not-a-timestamp"); err == nil {
			t.Error("expected an error for a malformed timestamp cursor value")
		}
	})

	t.Run("an unrecognized column is rejected", func(t *testing.T) {
		if _, err := parseColumnValue("some_other_column", "x"); err == nil {
			t.Error("expected an error for an unrecognized cursor column")
		}
	})
}

func TestBucketForListing(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		l    domain.Listing
		want int
	}{
		{"active -> bucket 0", domain.Listing{AuctionStart: now.Add(-time.Hour), AuctionEnd: now.Add(time.Hour)}, 0},
		{"upcoming -> bucket 1", domain.Listing{AuctionStart: now.Add(time.Hour), AuctionEnd: now.Add(2 * time.Hour)}, 1},
		{"ended -> bucket 2", domain.Listing{AuctionStart: now.Add(-2 * time.Hour), AuctionEnd: now.Add(-time.Hour)}, 2},
		{"purchased forces bucket 2 even mid-auction", func() domain.Listing {
			purchased := now.Add(-time.Minute)
			return domain.Listing{AuctionStart: now.Add(-time.Hour), AuctionEnd: now.Add(time.Hour), PurchasedAt: &purchased}
		}(), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bucketForListing(tc.l, now); got != tc.want {
				t.Errorf("bucketForListing() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCursorFor(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	base := domain.Listing{
		ID: "listing-1", CurrentPrice: 21_500, Year: 2025,
		AuctionStart: now.Add(time.Hour), AuctionEnd: now.Add(2 * time.Hour),
	}
	filter := listings.Filter{Make: "Mazda"}

	t.Run("price-low/price-high both encode CurrentPrice", func(t *testing.T) {
		for _, sort := range []listings.SortMode{listings.SortPriceLow, listings.SortPriceHigh} {
			c := cursorFor(base, sort, filter, now)
			if c.Value != "21500" || c.ID != "listing-1" || c.Fingerprint != listings.Fingerprint(sort, filter) {
				t.Errorf("cursorFor(%s) = %+v, want Value=21500", sort, c)
			}
		}
	})

	t.Run("year encodes Year", func(t *testing.T) {
		c := cursorFor(base, listings.SortYear, filter, now)
		if c.Value != "2025" {
			t.Errorf("cursorFor(year).Value = %q, want %q", c.Value, "2025")
		}
	})

	t.Run("ending sort in the upcoming bucket encodes AuctionStart, not AuctionEnd", func(t *testing.T) {
		c := cursorFor(base, listings.SortEnding, filter, now)
		if c.Bucket != 1 {
			t.Fatalf("Bucket = %d, want 1 (upcoming)", c.Bucket)
		}
		want := base.AuctionStart.UTC().Format(time.RFC3339Nano)
		if c.Value != want {
			t.Errorf("cursorFor(ending, upcoming).Value = %q, want %q (AuctionStart)", c.Value, want)
		}
	})

	t.Run("ending sort in the active bucket encodes AuctionEnd", func(t *testing.T) {
		active := base
		active.AuctionStart, active.AuctionEnd = now.Add(-time.Hour), now.Add(time.Hour)
		c := cursorFor(active, listings.SortEnding, filter, now)
		if c.Bucket != 0 {
			t.Fatalf("Bucket = %d, want 0 (active)", c.Bucket)
		}
		want := active.AuctionEnd.UTC().Format(time.RFC3339Nano)
		if c.Value != want {
			t.Errorf("cursorFor(ending, active).Value = %q, want %q (AuctionEnd)", c.Value, want)
		}
	})

	t.Run("ending sort in the ended bucket also encodes AuctionEnd", func(t *testing.T) {
		ended := base
		ended.AuctionStart, ended.AuctionEnd = now.Add(-2*time.Hour), now.Add(-time.Hour)
		c := cursorFor(ended, listings.SortEnding, filter, now)
		if c.Bucket != 2 {
			t.Fatalf("Bucket = %d, want 2 (ended)", c.Bucket)
		}
		want := ended.AuctionEnd.UTC().Format(time.RFC3339Nano)
		if c.Value != want {
			t.Errorf("cursorFor(ending, ended).Value = %q, want %q (AuctionEnd)", c.Value, want)
		}
	})
}

func TestReverseListings(t *testing.T) {
	cases := []struct {
		name string
		in   []domain.Listing
		want []string
	}{
		{"empty", nil, nil},
		{"single element is a no-op", []domain.Listing{{ID: "a"}}, []string{"a"}},
		{"even length", []domain.Listing{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, []string{"d", "c", "b", "a"}},
		{"odd length", []domain.Listing{{ID: "a"}, {ID: "b"}, {ID: "c"}}, []string{"c", "b", "a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reverseListings(tc.in)
			got := make([]string, len(tc.in))
			for i, l := range tc.in {
				got[i] = l.ID
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("reverseListings() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
