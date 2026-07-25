package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// InsertNewListings inserts every item that doesn't already exist (matched by id)
// and returns the ids that were actually newly inserted. Never updates or deletes an
// existing row (guidelines/06-backend-architecture.md, "Data lifecycle") --
// ON CONFLICT DO NOTHING is what makes "does this already exist" Postgres's own
// check rather than a separate SELECT-then-diff, which is both what makes this
// race-safe against a concurrent reconcile run and what makes leaving an existing
// row's current fields alone automatic, not something application code has to
// remember to skip. auction_end is deliberately not in the column list: the
// trg_set_auction_end trigger (migration 0001) computes it.
func InsertNewListings(ctx context.Context, pool *pgxpool.Pool, items []domain.Listing) ([]string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("pgstore: begin insert-new-listings tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	var inserted []string
	for _, item := range items {
		id, err := insertOneIfMissing(ctx, tx, item)
		if err != nil {
			return nil, fmt.Errorf("pgstore: insert listing %s: %w", item.ID, err)
		}
		if id != "" {
			inserted = append(inserted, id)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("pgstore: commit insert-new-listings tx: %w", err)
	}
	return inserted, nil
}

// insertOneIfMissing returns the row's id if it was newly inserted, or "" if a row
// with that id already existed (ON CONFLICT DO NOTHING -> no row returned).
func insertOneIfMissing(ctx context.Context, tx pgx.Tx, l domain.Listing) (string, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO listings (
			id, vin, year, make, model, trim, body_style, exterior_color, interior_color,
			engine, transmission, drivetrain, odometer_km, fuel_type, condition_grade,
			condition_report, damage_notes, title_status, province, city,
			auction_start, auction_duration_sec, starting_bid, reserve_price, buy_now_price,
			images, selling_dealership, lot, current_price, bid_count
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25,
			$26, $27, $28, $29, $30
		)
		ON CONFLICT (id) DO NOTHING
		RETURNING id`,
		l.ID, l.VIN, l.Year, l.Make, l.Model, l.Trim, l.BodyStyle, l.ExteriorColor, l.InteriorColor,
		l.Engine, l.Transmission, l.Drivetrain, l.OdometerKM, l.FuelType, l.ConditionGrade,
		l.ConditionReport, l.DamageNotes, l.TitleStatus, l.Province, l.City,
		l.AuctionStart, int(l.AuctionDuration.Seconds()), l.StartingBid, l.ReservePrice, l.BuyNowPrice,
		l.Images, l.SellingDealership, l.Lot, l.CurrentPrice, l.BidCount,
	)

	var id string
	if err := row.Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil // already existed -- left untouched, exactly as intended
		}
		return "", err
	}
	return id, nil
}
