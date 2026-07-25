//go:build integration

// Package integration holds tests that need a real Redis/Postgres (via
// testcontainers-go), kept out of the default `go test ./...` run so that gate stays
// fast and Docker-independent. Run explicitly: `go test -tags=integration ./test/integration/...`
// (guidelines/06-backend-architecture.md, "Go conventions").
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("theblock_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}

func TestMigrate_CreatesExpectedSchema(t *testing.T) {
	dsn := startPostgres(t)

	if err := pgstore.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() = %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	for _, table := range []string{"listings", "bids"} {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)`, table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %q: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migration", table)
		}
	}

	for _, idx := range []string{
		"idx_listings_auction_end", "idx_listings_auction_start", "idx_listings_price",
		"idx_listings_year", "idx_listings_make", "idx_bids_stream_dedup", "idx_bids_history",
	} {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT FROM pg_indexes WHERE indexname = $1)`, idx,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %q: %v", idx, err)
		}
		if !exists {
			t.Errorf("expected index %q to exist after migration", idx)
		}
	}

	// Migrate is safe to call again (cmd/reconcile calls it on every startup).
	if err := pgstore.Migrate(dsn); err != nil {
		t.Fatalf("second Migrate() call = %v, want nil (idempotent)", err)
	}
}

func TestMigrate_AuctionEndDerivesFromDurationColumn(t *testing.T) {
	dsn := startPostgres(t)
	if err := pgstore.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() = %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	const durationSec = 3600 // 1 hour -- deliberately not the 24h default, to prove it's read from the column

	_, err = pool.Exec(ctx, `
		INSERT INTO listings (
			id, vin, year, make, model, trim, body_style, exterior_color, interior_color,
			engine, transmission, drivetrain, odometer_km, fuel_type, condition_grade,
			condition_report, title_status, province, city,
			auction_start, auction_duration_sec, starting_bid, selling_dealership, lot, current_price
		) VALUES (
			gen_random_uuid(), 'VIN123', 2024, 'Mazda', 'CX-5', 'Turbo', 'SUV', 'Blue', 'Grey',
			'2.5L I4', 'automatic', 'FWD', 10000, 'gasoline', 4.0,
			'clean', 'clean', 'Ontario', 'Toronto',
			$1, $2, 10000, 'Test Dealer', 'A-1', 10000
		)`, start, durationSec)
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}

	var end time.Time
	if err := pool.QueryRow(ctx, `SELECT auction_end FROM listings WHERE vin = 'VIN123'`).Scan(&end); err != nil {
		t.Fatalf("select auction_end: %v", err)
	}

	want := start.Add(durationSec * time.Second)
	if !end.Equal(want) {
		t.Errorf("auction_end = %v, want %v (auction_start + auction_duration_sec)", end, want)
	}
}
