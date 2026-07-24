//go:build integration

package integration

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

func sampleListing(id, vin string) domain.Listing {
	reserve := int64(29_000)
	return domain.Listing{
		ID: id, VIN: vin, Year: 2025, Make: "Mazda", Model: "CX-5", Trim: "Turbo",
		BodyStyle: "SUV", ExteriorColor: "Blue", InteriorColor: "Light Grey",
		Engine: "2.5L I4", Transmission: "automatic", Drivetrain: "FWD",
		OdometerKM: 24_534, FuelType: domain.FuelGasoline, ConditionGrade: 4.0,
		ConditionReport: "Very clean.", DamageNotes: []string{},
		TitleStatus: domain.TitleClean, Province: "Ontario", City: "Mississauga",
		AuctionStart: time.Date(2026, 7, 28, 19, 0, 0, 0, time.UTC), AuctionDuration: 24 * time.Hour,
		StartingBid: 20_500, ReservePrice: &reserve, BuyNowPrice: nil,
		Images: []string{"https://placehold.co/800x600"}, SellingDealership: "Highway 7 Auto Sales", Lot: "A-0001",
		CurrentPrice: 20_500, BidCount: 0,
	}
}

func newPoolAndMigrate(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := startPostgres(t)
	if err := pgstore.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() = %v", err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestListingReader_Get(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	t.Run("not found", func(t *testing.T) {
		_, err := reader.Get(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Get(unknown id) error = %v, want domain.ErrNotFound", err)
		}
	})

	t.Run("returns what was inserted, including derived auction_end", func(t *testing.T) {
		want := sampleListing("11111111-1111-1111-1111-111111111111", "VINAAA111")
		if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{want}); err != nil {
			t.Fatalf("InsertNewListings: %v", err)
		}

		got, err := reader.Get(ctx, want.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		if got.VIN != want.VIN || got.Make != want.Make || got.CurrentPrice != want.CurrentPrice {
			t.Errorf("Get() = %+v, want fields matching %+v", got, want)
		}
		if got.ReservePrice == nil || *got.ReservePrice != *want.ReservePrice {
			t.Errorf("ReservePrice = %v, want %v (must round-trip even though it's excluded from the buyer DTO)", got.ReservePrice, *want.ReservePrice)
		}
		if got.BuyNowPrice != nil {
			t.Errorf("BuyNowPrice = %v, want nil", got.BuyNowPrice)
		}
		wantEnd := want.AuctionStart.Add(want.AuctionDuration)
		if !got.AuctionEnd.Equal(wantEnd) {
			t.Errorf("AuctionEnd = %v, want %v", got.AuctionEnd, wantEnd)
		}
	})
}

func TestInsertNewListings_InsertOnlyNew(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	a := sampleListing("22222222-2222-2222-2222-222222222222", "VINBBB222")
	b := sampleListing("33333333-3333-3333-3333-333333333333", "VINCCC333")

	t.Run("fresh insert returns both new ids", func(t *testing.T) {
		insertedIDs, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{a, b})
		if err != nil {
			t.Fatalf("InsertNewListings: %v", err)
		}
		if len(insertedIDs) != 2 {
			t.Errorf("inserted %d ids, want 2 (got %v)", len(insertedIDs), insertedIDs)
		}
	})

	t.Run("re-running with the same input inserts nothing", func(t *testing.T) {
		insertedIDs, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{a, b})
		if err != nil {
			t.Fatalf("InsertNewListings: %v", err)
		}
		if len(insertedIDs) != 0 {
			t.Errorf("inserted %d ids on a re-run of unchanged input, want 0 (got %v)", len(insertedIDs), insertedIDs)
		}
	})

	t.Run("a live bid on an existing row survives a reconcile run untouched", func(t *testing.T) {
		// Simulate what a real accepted bid would have done to listing a's row.
		if _, err := pool.Exec(ctx, `UPDATE listings SET current_price = 55555, bid_count = 7 WHERE id = $1`, a.ID); err != nil {
			t.Fatalf("simulate live bid: %v", err)
		}

		// Reconcile re-runs with a's ORIGINAL starting data (as if data/vehicles.json
		// were reloaded unchanged) plus one genuinely new listing, c.
		c := sampleListing("44444444-4444-4444-4444-444444444444", "VINDDD444")
		insertedIDs, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{a, b, c})
		if err != nil {
			t.Fatalf("InsertNewListings: %v", err)
		}
		if len(insertedIDs) != 1 || insertedIDs[0] != c.ID {
			t.Errorf("inserted ids = %v, want exactly [%s]", insertedIDs, c.ID)
		}

		got, err := reader.Get(ctx, a.ID)
		if err != nil {
			t.Fatalf("Get(a): %v", err)
		}
		if got.CurrentPrice != 55_555 || got.BidCount != 7 {
			t.Errorf("listing a after reconcile = current_price=%d bid_count=%d, want the live-bid values (55555, 7) untouched", got.CurrentPrice, got.BidCount)
		}
	})
}

// listAll runs a large-enough single ListPage call to behave like an
// unpaginated "give me everything matching this filter" query, sorted by
// year (an arbitrary but stable choice for a test that isn't exercising
// pagination or sort order itself).
func listAll(t *testing.T, ctx context.Context, reader *pgstore.ListingReader, filter listings.Filter) []domain.Listing {
	t.Helper()
	page, err := reader.ListPage(ctx, listings.PageRequest{Filter: filter, Sort: listings.SortYear, First: 50})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	return page.Items
}

func TestListingReader_FilterSemanticsAndDistinctMakes(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)
	now := time.Now()

	mazda := sampleListing("aaaaaaaa-0000-0000-0000-000000000001", "MAZDAVIN001")
	mazda.AuctionStart = now.Add(-time.Hour) // active
	mazda.AuctionDuration = 24 * time.Hour

	toyota := sampleListing("aaaaaaaa-0000-0000-0000-000000000002", "TOYOTAVIN002")
	toyota.Make, toyota.Model = "Toyota", "Camry"
	toyota.AuctionStart = now.Add(time.Hour) // upcoming
	toyota.AuctionDuration = 24 * time.Hour

	ended := sampleListing("aaaaaaaa-0000-0000-0000-000000000003", "ENDEDVIN003")
	ended.Make, ended.Model = "Toyota", "Corolla"
	ended.AuctionStart = now.Add(-48 * time.Hour) // ended (well past a 24h window)
	ended.AuctionDuration = 24 * time.Hour

	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{mazda, toyota, ended}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	t.Run("no filter returns everything", func(t *testing.T) {
		got := listAll(t, ctx, reader, listings.Filter{})
		if len(got) != 3 {
			t.Errorf("got %d listings, want 3", len(got))
		}
	})

	t.Run("make filter is an exact match", func(t *testing.T) {
		got := listAll(t, ctx, reader, listings.Filter{Make: "Toyota"})
		if len(got) != 2 {
			t.Fatalf("got %d listings for make=Toyota, want 2", len(got))
		}
		for _, l := range got {
			if l.Make != "Toyota" {
				t.Errorf("got make %q, want only Toyota", l.Make)
			}
		}
	})

	t.Run("status filter matches the real lifecycle at query time", func(t *testing.T) {
		for _, tt := range []struct {
			status   string
			wantVIN  string
			wantOnly bool
		}{
			{"active", "MAZDAVIN001", true},
			{"upcoming", "TOYOTAVIN002", true},
			{"ended", "ENDEDVIN003", true},
		} {
			got := listAll(t, ctx, reader, listings.Filter{Status: tt.status})
			if len(got) != 1 || got[0].VIN != tt.wantVIN {
				t.Errorf("status=%s returned %d listings (want [%s]): %+v", tt.status, len(got), tt.wantVIN, got)
			}
		}
	})

	t.Run("search matches across make/model/vin, case-insensitively", func(t *testing.T) {
		got := listAll(t, ctx, reader, listings.Filter{Search: "corolla"})
		if len(got) != 1 || got[0].VIN != "ENDEDVIN003" {
			t.Errorf("search=corolla returned %+v, want just ENDEDVIN003", got)
		}
	})

	t.Run("filters compose", func(t *testing.T) {
		got := listAll(t, ctx, reader, listings.Filter{Make: "Toyota", Status: "ended"})
		if len(got) != 1 || got[0].VIN != "ENDEDVIN003" {
			t.Errorf("make=Toyota+status=ended returned %+v, want just ENDEDVIN003", got)
		}
	})

	t.Run("DistinctMakes is sorted and deduplicated", func(t *testing.T) {
		makes, err := reader.DistinctMakes(ctx)
		if err != nil {
			t.Fatalf("DistinctMakes: %v", err)
		}
		if !sort.StringsAreSorted(makes) {
			t.Errorf("makes = %v, want sorted", makes)
		}
		if len(makes) != 2 { // Mazda, Toyota -- Toyota appears twice in the data but once in the facet
			t.Errorf("makes = %v, want exactly [Mazda, Toyota]", makes)
		}
	})
}
