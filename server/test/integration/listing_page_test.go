//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// priceListing is sampleListing (pgstore_test.go) with id/vin/current_price
// overridden -- everything else is irrelevant to pagination-order assertions.
func priceListing(id, vin string, price int64) domain.Listing {
	l := sampleListing(id, vin)
	l.CurrentPrice = price
	return l
}

func TestListPage_ForwardBackwardRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	items := []domain.Listing{
		priceListing("b0000000-0000-0000-0000-000000000001", "RTVIN001", 10_000),
		priceListing("b0000000-0000-0000-0000-000000000002", "RTVIN002", 20_000),
		priceListing("b0000000-0000-0000-0000-000000000003", "RTVIN003", 30_000),
		priceListing("b0000000-0000-0000-0000-000000000004", "RTVIN004", 40_000),
		priceListing("b0000000-0000-0000-0000-000000000005", "RTVIN005", 50_000),
	}
	if _, err := pgstore.InsertNewListings(ctx, pool, items); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	page1, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 2})
	if err != nil {
		t.Fatalf("ListPage page1: %v", err)
	}
	if len(page1.Items) != 2 || page1.Items[0].VIN != "RTVIN001" || page1.Items[1].VIN != "RTVIN002" {
		t.Fatalf("page1 = %+v, want [RTVIN001, RTVIN002]", page1.Items)
	}
	if !page1.HasNextPage || page1.HasPrevPage {
		t.Errorf("page1 page_info = {next:%v prev:%v}, want {next:true prev:false}", page1.HasNextPage, page1.HasPrevPage)
	}
	pivot := page1.EndCursor // the cursor for RTVIN002

	// Forward from the pivot reaches everything after it, in order.
	forward, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 10, After: pivot})
	if err != nil {
		t.Fatalf("ListPage forward-from-pivot: %v", err)
	}
	if len(forward.Items) != 3 || forward.Items[0].VIN != "RTVIN003" || forward.Items[2].VIN != "RTVIN005" {
		t.Fatalf("forward-from-pivot = %+v, want [RTVIN003, RTVIN004, RTVIN005]", forward.Items)
	}
	if forward.HasNextPage {
		t.Errorf("forward-from-pivot.HasNextPage = true, want false (that's everything after the pivot)")
	}

	// Backward from the identical pivot reaches exactly what precedes it -- proving
	// forward/backward are symmetric around the same cursor, not two independent codepaths
	// that happen to agree in the easy case (guidelines/06-backend-architecture.md, "Cursor pagination").
	backward, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, Last: 10, Before: pivot})
	if err != nil {
		t.Fatalf("ListPage backward-from-pivot: %v", err)
	}
	if len(backward.Items) != 1 || backward.Items[0].VIN != "RTVIN001" {
		t.Fatalf("backward-from-pivot = %+v, want [RTVIN001]", backward.Items)
	}
	if backward.HasPrevPage {
		t.Errorf("backward-from-pivot.HasPrevPage = true, want false (RTVIN001 is the very first row)")
	}
}

// TestListPage_EndingSortCrossesBuckets proves a single page can walk from one
// ending-sort bucket into the next (active -> upcoming -> ended) rather than
// stopping short at a bucket boundary (guidelines/06-backend-architecture.md,
// "Cursor pagination" -- the bucketed keyset walk).
func TestListPage_EndingSortCrossesBuckets(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)
	now := time.Now()

	active1 := sampleListing("c0000000-0000-0000-0000-000000000001", "ENDVIN001")
	active1.AuctionStart, active1.AuctionDuration = now.Add(-time.Hour), 2*time.Hour // active, ends soonest

	upcoming1 := sampleListing("c0000000-0000-0000-0000-000000000002", "ENDVIN002")
	upcoming1.AuctionStart, upcoming1.AuctionDuration = now.Add(time.Hour), 24*time.Hour // upcoming, starts soonest

	upcoming2 := sampleListing("c0000000-0000-0000-0000-000000000003", "ENDVIN003")
	upcoming2.AuctionStart, upcoming2.AuctionDuration = now.Add(2*time.Hour), 24*time.Hour

	ended1 := sampleListing("c0000000-0000-0000-0000-000000000004", "ENDVIN004")
	ended1.AuctionStart, ended1.AuctionDuration = now.Add(-48*time.Hour), 24*time.Hour // ended a day ago

	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{active1, upcoming1, upcoming2, ended1}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	page1, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortEnding, First: 2})
	if err != nil {
		t.Fatalf("ListPage page1: %v", err)
	}
	if len(page1.Items) != 2 || page1.Items[0].VIN != "ENDVIN001" || page1.Items[1].VIN != "ENDVIN002" {
		t.Fatalf("page1 = %+v, want [ENDVIN001 (active), ENDVIN002 (upcoming)] -- one page crossing active into upcoming", page1.Items)
	}
	if !page1.HasNextPage {
		t.Errorf("page1.HasNextPage = false, want true (upcoming2 + ended1 remain)")
	}

	page2, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortEnding, First: 2, After: page1.EndCursor})
	if err != nil {
		t.Fatalf("ListPage page2: %v", err)
	}
	if len(page2.Items) != 2 || page2.Items[0].VIN != "ENDVIN003" || page2.Items[1].VIN != "ENDVIN004" {
		t.Fatalf("page2 = %+v, want [ENDVIN003 (upcoming), ENDVIN004 (ended)] -- crossing again, upcoming into ended", page2.Items)
	}
	if page2.HasNextPage {
		t.Errorf("page2.HasNextPage = true, want false (that's all 4 rows)")
	}
}

func TestListPage_FingerprintMismatchRejected(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	a := priceListing("d0000000-0000-0000-0000-000000000001", "FPVIN001", 10_000)
	b := priceListing("d0000000-0000-0000-0000-000000000002", "FPVIN002", 20_000)
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{a, b}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	page, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 1})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	cursor := page.EndCursor

	t.Run("a different sort is rejected", func(t *testing.T) {
		_, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceHigh, First: 1, After: cursor})
		if !errors.Is(err, domain.ErrCursorSortMismatch) {
			t.Errorf("error = %v, want domain.ErrCursorSortMismatch", err)
		}
	})

	t.Run("a different filter is rejected", func(t *testing.T) {
		_, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, Filter: listings.Filter{Make: "Toyota"}, First: 1, After: cursor})
		if !errors.Is(err, domain.ErrCursorSortMismatch) {
			t.Errorf("error = %v, want domain.ErrCursorSortMismatch", err)
		}
	})

	t.Run("the same sort and filter is accepted", func(t *testing.T) {
		if _, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 1, After: cursor}); err != nil {
			t.Errorf("ListPage with matching sort+filter = %v, want nil", err)
		}
	})
}

// TestListPage_TiebreakWithDuplicateSortValues proves ties on the sort column
// (real with only 200 rows -- guidelines/06-backend-architecture.md) get a
// deterministic secondary order (id) instead of an arbitrary one that could
// skip or duplicate a row across a page boundary.
func TestListPage_TiebreakWithDuplicateSortValues(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	tie := []domain.Listing{
		priceListing("e0000000-0000-0000-0000-000000000003", "TIEVIN003", 25_000),
		priceListing("e0000000-0000-0000-0000-000000000001", "TIEVIN001", 25_000),
		priceListing("e0000000-0000-0000-0000-000000000002", "TIEVIN002", 25_000),
	}
	if _, err := pgstore.InsertNewListings(ctx, pool, tie); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	page1, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 2})
	if err != nil {
		t.Fatalf("ListPage page1: %v", err)
	}
	if len(page1.Items) != 2 ||
		page1.Items[0].ID != "e0000000-0000-0000-0000-000000000001" ||
		page1.Items[1].ID != "e0000000-0000-0000-0000-000000000002" {
		t.Fatalf("page1 = %+v, want ids .001 then .002 -- equal prices must tiebreak by id ascending", page1.Items)
	}

	page2, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 2, After: page1.EndCursor})
	if err != nil {
		t.Fatalf("ListPage page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].ID != "e0000000-0000-0000-0000-000000000003" {
		t.Fatalf("page2 = %+v, want exactly id .003 -- no skip and no repeat of .001/.002 across the boundary", page2.Items)
	}
	if page2.HasNextPage {
		t.Errorf("page2.HasNextPage = true, want false")
	}
}

// TestListPage_CursorWalkToleratesReorderingBetweenFetches is the core case the
// whole cursor design exists for (guidelines/06-backend-architecture.md): a sort
// key changing between two page fetches must never skip or duplicate a row the
// way OFFSET/LIMIT would.
func TestListPage_CursorWalkToleratesReorderingBetweenFetches(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewListingReader(pool)

	items := []domain.Listing{
		priceListing("f0000000-0000-0000-0000-000000000001", "REORDVIN001", 10_000),
		priceListing("f0000000-0000-0000-0000-000000000002", "REORDVIN002", 20_000),
		priceListing("f0000000-0000-0000-0000-000000000003", "REORDVIN003", 30_000),
		priceListing("f0000000-0000-0000-0000-000000000004", "REORDVIN004", 40_000),
	}
	if _, err := pgstore.InsertNewListings(ctx, pool, items); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	page1, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 2})
	if err != nil {
		t.Fatalf("ListPage page1: %v", err)
	}
	if len(page1.Items) != 2 || page1.Items[0].VIN != "REORDVIN001" || page1.Items[1].VIN != "REORDVIN002" {
		t.Fatalf("page1 = %+v, want [REORDVIN001, REORDVIN002]", page1.Items)
	}

	// Simulate a bid landing on REORDVIN004 between the two page fetches: it now
	// sorts BEFORE the cursor's own position (5000 < the pivot's 20000).
	if _, err := pool.Exec(ctx, `UPDATE listings SET current_price = 5000 WHERE vin = 'REORDVIN004'`); err != nil {
		t.Fatalf("simulate reordering bid: %v", err)
	}

	page2, err := reader.ListPage(ctx, listings.PageRequest{Sort: listings.SortPriceLow, First: 2, After: page1.EndCursor})
	if err != nil {
		t.Fatalf("ListPage page2: %v", err)
	}
	// The cursor is a predicate against current data ((current_price, id) > the
	// pivot's), not a positional offset -- so REORDVIN004 having re-sorted earlier
	// than the pivot correctly excludes it here instead of re-showing it.
	if len(page2.Items) != 1 || page2.Items[0].VIN != "REORDVIN003" {
		t.Fatalf("page2 = %+v, want exactly [REORDVIN003] -- REORDVIN004 re-sorted before the pivot and must not reappear", page2.Items)
	}
	if page2.HasNextPage {
		t.Errorf("page2.HasNextPage = true, want false")
	}
}
