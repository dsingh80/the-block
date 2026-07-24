// Command reconcile is the one-shot startup task (a Compose init service, see
// guidelines/06-backend-architecture.md) that applies pending Postgres migrations,
// syncs data/vehicles.json into the listings table -- insert-only-new, never
// updating or deleting an existing row -- and primes any missing Redis listing
// state from what Postgres now holds.
package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/platform/config"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/platform/seeddata"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	if err := pgstore.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("reconcile: migrate: %v", err)
	}

	vehicles, err := seeddata.LoadVehicles(cfg.VehiclesDataPath)
	if err != nil {
		log.Fatalf("reconcile: load %s: %v", cfg.VehiclesDataPath, err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("reconcile: connect to postgres: %v", err)
	}
	defer pool.Close()

	inserted, err := pgstore.InsertNewListings(ctx, pool, vehicles)
	if err != nil {
		log.Fatalf("reconcile: insert new listings: %v", err)
	}
	log.Printf("reconcile: %d listing(s) in %s, %d newly inserted, %d already present and left untouched",
		len(vehicles), cfg.VehiclesDataPath, len(inserted), len(vehicles)-len(inserted))

	// Redis priming reads back from Postgres, not the seeddata-loaded slice --
	// only Postgres has auction_end as the trigger actually computed it, and
	// only Postgres reflects real bidding a recovery scenario needs to restore.
	reader := pgstore.NewListingReader(pool)
	allListings, err := reader.ListAll(ctx)
	if err != nil {
		log.Fatalf("reconcile: list all listings: %v", err)
	}

	rdb := redisstore.NewClient(cfg.RedisAddr)
	defer rdb.Close()

	for _, l := range allListings {
		if err := redisstore.PrimeListingState(ctx, rdb, l); err != nil {
			log.Fatalf("reconcile: prime redis state for listing %s: %v", l.ID, err)
		}
	}
	log.Printf("reconcile: primed/verified Redis state for all %d listings", len(allListings))
}
