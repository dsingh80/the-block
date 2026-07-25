// Package pgstore holds the Postgres system of record: the durable, queryable
// data (listings, bids) behind the Redis hot path (guidelines/06-backend-architecture.md).
package pgstore

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies every pending migration against the database at dsn. Safe to
// call on every startup (part of cmd/reconcile, a later commit) -- it's a no-op
// once the schema is already current.
func Migrate(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("pgstore: load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("pgstore: init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("pgstore: apply migrations: %w", err)
	}
	return nil
}
