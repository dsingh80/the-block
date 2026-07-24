package redisstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// StreamTailer drains every listing's Redis stream into Postgres (the durable
// bid audit trail) and republishes each accepted entry to the Broadcaster
// (guidelines/06-backend-architecture.md). Draining and broadcasting are two
// different distribution patterns off the same stream, both handled by this one
// in-process tailer: correct and sufficient at single-instance scale, where a
// consumer group would be complexity this doesn't need yet.
type StreamTailer struct {
	rdb         *redis.Client
	pool        *pgxpool.Pool
	broadcaster realtime.Broadcaster
}

func NewStreamTailer(rdb *redis.Client, pool *pgxpool.Pool, broadcaster realtime.Broadcaster) *StreamTailer {
	return &StreamTailer{rdb: rdb, pool: pool, broadcaster: broadcaster}
}

// Run polls at interval until ctx is cancelled. A per-tick error is logged and
// the loop continues -- a transient Redis/Postgres hiccup shouldn't permanently
// stop draining, since the atomic accept path in Redis keeps working regardless.
func (t *StreamTailer) Run(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := t.Tick(ctx); err != nil {
				slog.Error("stream tailer tick failed", "error", err)
			}
		}
	}
}

// Tick performs one drain pass across every listing's stream, resuming each
// from its own last_event_stream_id checkpoint (stored on the listings row).
func (t *StreamTailer) Tick(ctx context.Context) error {
	rows, err := t.pool.Query(ctx, `SELECT id, COALESCE(last_event_stream_id, '0') FROM listings`)
	if err != nil {
		return fmt.Errorf("redisstore: stream tailer: list listings: %w", err)
	}

	streamKeyToListingID := make(map[string]string)
	streamsArg := make([]string, 0)
	var startIDs []string
	for rows.Next() {
		var listingID, lastID string
		if err := rows.Scan(&listingID, &lastID); err != nil {
			rows.Close()
			return fmt.Errorf("redisstore: stream tailer: scan checkpoint: %w", err)
		}
		key := ListingStreamKey(listingID)
		streamKeyToListingID[key] = listingID
		streamsArg = append(streamsArg, key)
		startIDs = append(startIDs, lastID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("redisstore: stream tailer: iterate checkpoints: %w", err)
	}
	if len(streamsArg) == 0 {
		return nil
	}
	streamsArg = append(streamsArg, startIDs...)

	// Block must be negative, not the zero value: go-redis only omits the BLOCK
	// argument when Block < 0. Block: 0 (the zero value a bare struct literal
	// would have) is sent to Redis as a literal "BLOCK 0", which means block
	// forever, not "don't block" -- confirmed by hitting exactly that hang
	// (a Tick call that never returned) before adding this explicitly.
	result, err := t.rdb.XRead(ctx, &redis.XReadArgs{Streams: streamsArg, Count: 100, Block: -1}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil // nothing new on any stream
		}
		return fmt.Errorf("redisstore: stream tailer: xread: %w", err)
	}

	for _, stream := range result {
		listingID := streamKeyToListingID[stream.Stream]
		for _, msg := range stream.Messages {
			if err := t.processEntry(ctx, listingID, msg); err != nil {
				slog.Error("stream tailer: process entry failed",
					"listing_id", listingID, "entry_id", msg.ID, "error", err)
			}
		}
	}
	return nil
}

func (t *StreamTailer) processEntry(ctx context.Context, listingID string, msg redis.XMessage) error {
	bidID, _ := msg.Values["bid_id"].(string)
	entryType, _ := msg.Values["type"].(string)
	sessionID, _ := msg.Values["session_id"].(string)

	amount, err := parseFieldInt64(msg.Values["amount"])
	if err != nil {
		return fmt.Errorf("parse amount: %w", err)
	}
	bidCount, err := parseFieldInt64(msg.Values["bid_count"])
	if err != nil {
		return fmt.Errorf("parse bid_count: %w", err)
	}
	acceptedAtMs, err := parseFieldInt64(msg.Values["accepted_at_ms"])
	if err != nil {
		return fmt.Errorf("parse accepted_at_ms: %w", err)
	}
	acceptedAt := time.UnixMilli(acceptedAtMs).UTC()

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	tag, err := tx.Exec(ctx, `
		INSERT INTO bids (id, listing_id, session_id, type, amount, bid_count_after, accepted_at, source_stream_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (listing_id, source_stream_id) DO NOTHING`,
		bidID, listingID, sessionID, entryType, amount, bidCount, acceptedAt, msg.ID)
	if err != nil {
		return fmt.Errorf("insert bid row: %w", err)
	}

	fresh := tag.RowsAffected() > 0
	if fresh {
		if entryType == "buy_now" {
			_, err = tx.Exec(ctx, `
				UPDATE listings SET current_price=$1, bid_count=$2, high_bidder_session_id=$3,
					purchased_at=$4, last_event_stream_id=$5, updated_at=now()
				WHERE id=$6`,
				amount, bidCount, sessionID, acceptedAt, msg.ID, listingID)
		} else {
			_, err = tx.Exec(ctx, `
				UPDATE listings SET current_price=$1, bid_count=$2, high_bidder_session_id=$3,
					last_event_stream_id=$4, updated_at=now()
				WHERE id=$5`,
				amount, bidCount, sessionID, msg.ID, listingID)
		}
	} else {
		// Already seen (defensive -- shouldn't normally happen, since each tick
		// re-reads the checkpoint fresh before requesting anything past it).
		// Still advance the checkpoint so the tailer can't loop on this entry
		// forever, but don't re-apply field values that could roll back a
		// genuinely newer bid processed since.
		_, err = tx.Exec(ctx, `UPDATE listings SET last_event_stream_id=$1 WHERE id=$2`, msg.ID, listingID)
	}
	if err != nil {
		return fmt.Errorf("update listing: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if fresh {
		event := realtime.Event{
			ListingID: listingID, BidID: bidID, CurrentPrice: amount, BidCount: int(bidCount),
			HighBidderSessionID: sessionID, AcceptedAt: acceptedAt,
		}
		if entryType == "buy_now" {
			// Time-based lifecycle ending (auction_end passing) is deliberately
			// not pushed here -- every connected client can already derive that
			// locally from auction_end, exactly like the existing client's own
			// clock store does today. Buy Now is a genuine discrete event with
			// no other way for a viewer to know, so it's the one thing this
			// tailer proactively announces as listing_ended
			// (guidelines/06-backend-architecture.md).
			event.Type = "listing_ended"
			event.Reason = "bought_now"
		} else {
			event.Type = "bid_accepted"
		}
		t.broadcaster.Publish(ctx, event)
	}

	return nil
}

func parseFieldInt64(v any) (int64, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("expected a string field value, got %T", v)
	}
	return strconv.ParseInt(s, 10, 64)
}
