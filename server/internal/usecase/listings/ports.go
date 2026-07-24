// Package listings is the listings use-case: the port(s) HTTP handlers depend on,
// implemented by internal/platform/pgstore (Postgres is the system of record for
// listing reads -- guidelines/06-backend-architecture.md). Kept free of net/http and
// pgx types so it isn't tied to either transport or storage.
package listings

import (
	"context"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Filter is the basic filter set: empty string means "no constraint" for that
// field. Composes with cursor pagination below, not a separate query shape
// (guidelines/06-backend-architecture.md).
type Filter struct {
	Status string // "", "upcoming", "active", "ended"
	Make   string // "", or an exact match
	Search string // "", or a substring match across year/make/model/trim/vin/selling_dealership
}

// PageRequest is one page of a cursor walk. Exactly one of (First+After) or
// (Last+Before) is meaningful at a time -- After/Before empty means "the
// first page in this direction." Both a forward and backward cursor being
// present is treated as forward taking precedence (a handler shouldn't send
// both, but the zero-value-friendly shape doesn't forbid it outright).
type PageRequest struct {
	Filter Filter
	Sort   SortMode
	First  int
	After  string
	Last   int
	Before string
}

// Page is one page of results plus enough to keep walking in either
// direction (guidelines/06-backend-architecture.md, "Cursor pagination").
type Page struct {
	Items       []domain.Listing
	HasNextPage bool
	HasPrevPage bool
	StartCursor string
	EndCursor   string
}

// Reader is the read-side port for listings.
type Reader interface {
	Get(ctx context.Context, id string) (domain.Listing, error)
	DistinctMakes(ctx context.Context) ([]string, error)
	ListPage(ctx context.Context, req PageRequest) (Page, error)
}
