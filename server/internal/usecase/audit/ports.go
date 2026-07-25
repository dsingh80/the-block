// Package audit is the bid-history use-case: the read-only port behind
// GET /v1/listings/{id}/bids (guidelines/06-backend-architecture.md). Kept free
// of net/http and pgx types, like every other usecase port.
package audit

import (
	"context"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Reader is the bid-history read port.
type Reader interface {
	// ListForListing returns up to limit of listingID's recorded bids, oldest
	// first. Chronological order is required, not incidental: it's what lets a
	// caller assign anonymized handles by true first-appearance
	// (guidelines/06-backend-architecture.md, "Bid-history anonymization"),
	// which a newest-first order would get backwards.
	ListForListing(ctx context.Context, listingID string, limit int) ([]domain.Bid, error)
}
