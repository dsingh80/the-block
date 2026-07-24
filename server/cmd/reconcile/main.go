// Command reconcile is the one-shot startup task (a Compose init service, see
// guidelines/06-backend-architecture.md) that applies pending Postgres migrations
// and syncs data/vehicles.json into the listings table -- insert-only-new, never
// updating or deleting an existing row. Redis state priming lands in a later
// commit, once a Redis client exists.
package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/platform/config"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
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
}
