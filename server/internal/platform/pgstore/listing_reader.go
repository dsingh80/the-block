package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// ListingReader implements listings.Reader (internal/usecase/listings) against
// Postgres, the system of record for listing reads (guidelines/06-backend-architecture.md).
type ListingReader struct {
	pool *pgxpool.Pool
}

func NewListingReader(pool *pgxpool.Pool) *ListingReader {
	return &ListingReader{pool: pool}
}

const listingColumns = `
	id, vin, year, make, model, trim, body_style, exterior_color, interior_color,
	engine, transmission, drivetrain, odometer_km, fuel_type, condition_grade,
	condition_report, damage_notes, title_status, province, city,
	auction_start, auction_duration_sec, auction_end, starting_bid, reserve_price,
	buy_now_price, images, selling_dealership, lot,
	current_price, bid_count, high_bidder_session_id, purchased_at`

func (r *ListingReader) Get(ctx context.Context, id string) (domain.Listing, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+listingColumns+` FROM listings WHERE id = $1`, id)

	l, err := scanListing(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Listing{}, domain.ErrNotFound
		}
		return domain.Listing{}, fmt.Errorf("pgstore: get listing %s: %w", id, err)
	}
	return l, nil
}

// ListAll returns every listing, ordered by id. A full scan is fine at this
// dataset's size (a couple hundred rows); it exists specifically for
// cmd/reconcile's Redis-priming pass, which needs auction_end as Postgres's
// trigger actually computed it -- not the seeddata-loaded value, which has no
// AuctionEnd set at all until Postgres derives it.
func (r *ListingReader) ListAll(ctx context.Context) ([]domain.Listing, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+listingColumns+` FROM listings ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list all listings: %w", err)
	}
	defer rows.Close()

	var listings []domain.Listing
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, fmt.Errorf("pgstore: scan listing: %w", err)
		}
		listings = append(listings, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: iterate listings: %w", err)
	}
	return listings, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow, single row) and pgx.Rows
// (Query, iterated via Next()) -- scanListing works for either.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanListing(row rowScanner) (domain.Listing, error) {
	var l domain.Listing
	var durationSec int

	err := row.Scan(
		&l.ID, &l.VIN, &l.Year, &l.Make, &l.Model, &l.Trim, &l.BodyStyle, &l.ExteriorColor, &l.InteriorColor,
		&l.Engine, &l.Transmission, &l.Drivetrain, &l.OdometerKM, &l.FuelType, &l.ConditionGrade,
		&l.ConditionReport, &l.DamageNotes, &l.TitleStatus, &l.Province, &l.City,
		&l.AuctionStart, &durationSec, &l.AuctionEnd, &l.StartingBid, &l.ReservePrice,
		&l.BuyNowPrice, &l.Images, &l.SellingDealership, &l.Lot,
		&l.CurrentPrice, &l.BidCount, &l.HighBidderSessionID, &l.PurchasedAt,
	)
	if err != nil {
		return domain.Listing{}, err
	}
	l.AuctionDuration = time.Duration(durationSec) * time.Second
	return l, nil
}
