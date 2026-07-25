//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/seeddata"
)

// End-to-end sanity check against the real, committed dataset (not just the small
// testdata fixture in internal/platform/seeddata) -- proves the full
// load -> migrate -> insert-only-new path handles all 200 real records, not just
// the couple of shapes a hand-written fixture happens to cover.
func TestReconcile_RealDataset(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)

	vehicles, err := seeddata.LoadVehicles("../../../data/vehicles.json")
	if err != nil {
		t.Fatalf("LoadVehicles(real dataset): %v", err)
	}
	if len(vehicles) != 200 {
		t.Fatalf("got %d vehicles from the real dataset, want 200", len(vehicles))
	}

	inserted, err := pgstore.InsertNewListings(ctx, pool, vehicles)
	if err != nil {
		t.Fatalf("InsertNewListings(real dataset): %v", err)
	}
	if len(inserted) != 200 {
		t.Errorf("inserted %d of 200 real listings, want all 200 on a fresh database", len(inserted))
	}

	reader := pgstore.NewListingReader(pool)
	got, err := reader.Get(ctx, vehicles[0].ID)
	if err != nil {
		t.Fatalf("Get(first real listing): %v", err)
	}
	if got.VIN != vehicles[0].VIN {
		t.Errorf("round-tripped VIN = %q, want %q", got.VIN, vehicles[0].VIN)
	}

	// Re-running against the now-populated database inserts nothing new.
	insertedAgain, err := pgstore.InsertNewListings(ctx, pool, vehicles)
	if err != nil {
		t.Fatalf("InsertNewListings(second run): %v", err)
	}
	if len(insertedAgain) != 0 {
		t.Errorf("second run inserted %d listings, want 0", len(insertedAgain))
	}
}
