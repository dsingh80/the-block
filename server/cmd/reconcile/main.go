// Command reconcile is the one-shot startup task (a Compose init service, see
// guidelines/06-backend-architecture.md) that applies pending Postgres migrations,
// syncs data/vehicles.json into the listings table -- insert-only-new, never
// updating or deleting an existing row -- and primes any missing Redis listing
// state from what Postgres now holds.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/platform/config"
	"github.com/dsingh80/the-block/server/internal/platform/logging"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/platform/seeddata"
)

func main() {
	slog.SetDefault(logging.New(os.Stdout))

	cfg := config.Load()
	ctx := context.Background()

	if err := pgstore.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("reconcile: migrate failed", "error", err)
		os.Exit(1)
	}

	vehicles, err := seeddata.LoadVehicles(cfg.VehiclesDataPath)
	if err != nil {
		slog.Error("reconcile: load vehicles failed", "path", cfg.VehiclesDataPath, "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("reconcile: connect to postgres failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	inserted, err := pgstore.InsertNewListings(ctx, pool, vehicles)
	if err != nil {
		slog.Error("reconcile: insert new listings failed", "error", err)
		os.Exit(1)
	}
	slog.Info("reconcile: dataset sync complete",
		"total_listings", len(vehicles), "path", cfg.VehiclesDataPath,
		"newly_inserted", len(inserted), "already_present", len(vehicles)-len(inserted))

	// Redis priming reads back from Postgres, not the seeddata-loaded slice --
	// only Postgres has auction_end as the trigger actually computed it, and
	// only Postgres reflects real bidding a recovery scenario needs to restore.
	reader := pgstore.NewListingReader(pool)
	allListings, err := reader.ListAll(ctx)
	if err != nil {
		slog.Error("reconcile: list all listings failed", "error", err)
		os.Exit(1)
	}

	rdb := redisstore.NewClient(cfg.RedisAddr)
	defer rdb.Close()

	for _, l := range allListings {
		if err := redisstore.PrimeListingState(ctx, rdb, l); err != nil {
			slog.Error("reconcile: prime redis state failed", "listing_id", l.ID, "error", err)
			os.Exit(1)
		}
	}
	slog.Info("reconcile: redis state primed", "listing_count", len(allListings))
}
