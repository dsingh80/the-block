// Package listings is the listings use-case: the port(s) HTTP handlers depend on,
// implemented by internal/platform/pgstore (Postgres is the system of record for
// listing reads -- guidelines/06-backend-architecture.md). Kept free of net/http and
// pgx types so it isn't tied to either transport or storage.
package listings

import (
	"context"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Filter is the basic (pre-pagination) filter set: empty string means "no
// constraint" for that field. Cursor pagination composes with this the same
// way in a later commit, not a separate query shape
// (guidelines/06-backend-architecture.md).
type Filter struct {
	Status string // "", "upcoming", "active", "ended"
	Make   string // "", or an exact match
	Search string // "", or a substring match across year/make/model/trim/vin/selling_dealership
}

// Reader is the read-side port for listings.
type Reader interface {
	Get(ctx context.Context, id string) (domain.Listing, error)
	ListFiltered(ctx context.Context, filter Filter) ([]domain.Listing, error)
	DistinctMakes(ctx context.Context) ([]string, error)
}
