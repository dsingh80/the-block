//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
)

// countingTracer counts how many SQL statements a pgxpool.Pool issues --
// the real-SQL half of proving GET /v1/listings/{id} never grows a join
// (guidelines/06-backend-architecture.md, "Computing viewer without joins").
// The handler-level test for the same claim (internal/transport/http/handlers)
// only proves the fake was called once; this proves the real query is, too.
type countingTracer struct {
	mu sync.Mutex
	n  int
}

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return ctx
}

func (c *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (c *countingTracer) reset() {
	c.mu.Lock()
	c.n = 0
	c.mu.Unlock()
}

func (c *countingTracer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestListingReader_Get_IssuesExactlyOneQuery(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(t)
	if err := pgstore.Migrate(dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	tracer := &countingTracer{}
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)

	listing := sampleListing("99999999-0000-0000-0000-000000000099", "QCOUNTVIN001")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}
	tracer.reset() // only count what Get() itself issues, not the setup above

	reader := pgstore.NewListingReader(pool)
	if _, err := reader.Get(ctx, listing.ID); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got := tracer.count(); got != 1 {
		t.Errorf("Get() issued %d SQL statements, want exactly 1 -- viewer fields must come from a "+
			"column comparison already on this row plus a Redis lookup the handler does separately, never a join or a second query", got)
	}
}

func insertBid(t *testing.T, pool *pgxpool.Pool, id, listingID, sessionID, bidType string, amount int64, bidCountAfter int, acceptedAt time.Time, streamID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO bids (id, listing_id, session_id, type, amount, bid_count_after, accepted_at, source_stream_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, listingID, sessionID, bidType, amount, bidCountAfter, acceptedAt, streamID)
	if err != nil {
		t.Fatalf("insert bid %s: %v", id, err)
	}
}

func TestBidReader_ListForListing_ReturnsChronologicalOrder(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)

	listing := sampleListing("99999999-0000-0000-0000-000000000001", "BIDREADVIN001")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// Inserted newest-first on purpose -- ListForListing's own ORDER BY must be
	// what produces chronological output, not insertion order happening to agree.
	insertBid(t, pool, "b0000000-0000-0000-0000-00000000a002", listing.ID, "session-b", "bid", 21_500, 2, t0.Add(time.Minute), "stream-2")
	insertBid(t, pool, "b0000000-0000-0000-0000-00000000a001", listing.ID, "session-a", "bid", 21_000, 1, t0, "stream-1")

	reader := pgstore.NewBidReader(pool)
	got, err := reader.ListForListing(ctx, listing.ID, 10)
	if err != nil {
		t.Fatalf("ListForListing: %v", err)
	}
	if len(got) != 2 || got[0].SessionID != "session-a" || got[1].SessionID != "session-b" {
		t.Fatalf("got = %+v, want [session-a, session-b] oldest first", got)
	}
	if got[0].Amount != 21_000 || got[1].Amount != 21_500 {
		t.Errorf("amounts = [%d, %d], want [21000, 21500]", got[0].Amount, got[1].Amount)
	}
}

func TestBidReader_ListForListing_RespectsLimitAndListingScope(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)

	a := sampleListing("99999999-0000-0000-0000-000000000002", "BIDREADVIN002")
	b := sampleListing("99999999-0000-0000-0000-000000000003", "BIDREADVIN003")
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{a, b}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}

	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	insertBid(t, pool, "b0000000-0000-0000-0000-00000000b001", a.ID, "session-a", "bid", 21_000, 1, t0, "stream-1")
	insertBid(t, pool, "b0000000-0000-0000-0000-00000000b002", a.ID, "session-a", "bid", 21_500, 2, t0.Add(time.Minute), "stream-2")
	insertBid(t, pool, "b0000000-0000-0000-0000-00000000b003", b.ID, "session-c", "bid", 30_000, 1, t0, "stream-3")

	reader := pgstore.NewBidReader(pool)

	limited, err := reader.ListForListing(ctx, a.ID, 1)
	if err != nil {
		t.Fatalf("ListForListing(a, limit=1): %v", err)
	}
	if len(limited) != 1 || limited[0].Amount != 21_000 {
		t.Fatalf("got = %+v, want exactly listing a's oldest bid, limited to 1", limited)
	}

	scoped, err := reader.ListForListing(ctx, b.ID, 10)
	if err != nil {
		t.Fatalf("ListForListing(b): %v", err)
	}
	if len(scoped) != 1 || scoped[0].SessionID != "session-c" {
		t.Fatalf("got = %+v, want only listing b's own bid, not listing a's", scoped)
	}
}

func TestBidReader_ListForListing_CanceledContextReturnsError(t *testing.T) {
	pool := newPoolAndMigrate(t)
	reader := pgstore.NewBidReader(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := reader.ListForListing(ctx, "any-id", 10); err == nil {
		t.Fatal("expected an error for ListForListing called with an already-canceled context, got nil")
	}
}
