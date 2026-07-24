// Package listings is the listings use-case: the port(s) HTTP handlers depend on,
// implemented by internal/platform/pgstore (Postgres is the system of record for
// listing reads -- guidelines/06-backend-architecture.md). Kept free of net/http and
// pgx types so it isn't tied to either transport or storage.
package listings

import (
	"context"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Reader is the read-side port for listings.
type Reader interface {
	Get(ctx context.Context, id string) (domain.Listing, error)
}
