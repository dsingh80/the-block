package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// BidReader implements audit.Reader (internal/usecase/audit) against Postgres's
// append-only bids table (guidelines/06-backend-architecture.md).
type BidReader struct {
	pool *pgxpool.Pool
}

func NewBidReader(pool *pgxpool.Pool) *BidReader {
	return &BidReader{pool: pool}
}

// ListForListing queries oldest-first: anonymized-handle assignment needs true
// chronological order (guidelines/06-backend-architecture.md, "Bid-history
// anonymization"), even though idx_bids_history is built DESC for the more
// common "most recent first" read direction -- the same btree index serves an
// ASC scan just as well, so this costs nothing extra.
func (r *BidReader) ListForListing(ctx context.Context, listingID string, limit int) ([]domain.Bid, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, listing_id, session_id, COALESCE(request_id, ''), type, amount, bid_count_after, accepted_at, source_stream_id
		FROM bids
		WHERE listing_id = $1
		ORDER BY accepted_at ASC, id ASC
		LIMIT $2`, listingID, limit)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list bids for listing %s: %w", listingID, err)
	}
	defer rows.Close()

	result := make([]domain.Bid, 0, limit)
	for rows.Next() {
		var b domain.Bid
		if err := rows.Scan(&b.ID, &b.ListingID, &b.SessionID, &b.RequestID, &b.Type, &b.Amount, &b.BidCountAfter, &b.AcceptedAt, &b.SourceStreamID); err != nil {
			return nil, fmt.Errorf("pgstore: scan bid: %w", err)
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: iterate bids: %w", err)
	}
	return result, nil
}
