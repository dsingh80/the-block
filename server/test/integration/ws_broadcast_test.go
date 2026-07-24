//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/inmemory"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	"github.com/dsingh80/the-block/server/internal/transport/ws"
)

// jsonCapturingWriter captures each WriteJSON call as raw JSON bytes. ws's own
// wire message types are unexported, so a test outside that package verifies
// shape via json tags -- the actual wire contract -- instead of a type
// assertion, the same way an HTTP handler test decodes a response body.
type jsonCapturingWriter struct {
	sent [][]byte
}

func (w *jsonCapturingWriter) WriteJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.sent = append(w.sent, b)
	return nil
}

// TestWSHub_RealBidThroughUseCaseLayerReachesASubscribedConnection is the
// full-stack version of a claim otherwise only proven piecewise: BidStore's
// own accept-path correctness (bidding_test.go), StreamTailer draining +
// broadcasting to a bare realtime.Subscriber (stream_tailer_test.go), and
// Connection's own event-to-wire-message translation given a manually
// constructed Event (ws/connection_test.go). Here all of it runs together for
// real: a bid accepted through the same Redis Lua path the HTTP handler calls,
// drained by a real StreamTailer, broadcast by a real Broadcaster, delivered
// to a real ws.Connection -- a fake writer standing in for the network socket
// (the socket itself is already covered end-to-end in ws/hub_test.go) -- and
// the message it actually receives is checked against the real wire contract
// (guidelines/06-backend-architecture.md, "WebSocket protocol").
func TestWSHub_RealBidThroughUseCaseLayerReachesASubscribedConnection(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)

	listing := sampleListing("88888888-8888-8888-8888-888888888888", "VINWSBID1")
	listing.AuctionStart = time.Now().Add(-time.Hour)
	listing.AuctionDuration = 24 * time.Hour
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}
	fromPostgres, err := pgstore.NewListingReader(pool).Get(ctx, listing.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := redisstore.PrimeListingState(ctx, rdb, fromPostgres); err != nil {
		t.Fatalf("PrimeListingState: %v", err)
	}

	broadcaster := inmemory.NewBroadcaster()
	writer := &jsonCapturingWriter{}
	conn := ws.NewConnection(writer, "session-viewer") // watching, not the one bidding
	broadcaster.Subscribe(conn, listing.ID)

	bidStore := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	if _, err := bidStore.PlaceBid(ctx, listing.ID, "session-bidder", 21_000); err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}

	tailer := redisstore.NewStreamTailer(rdb, pool, broadcaster)
	if err := tailer.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(writer.sent) != 1 {
		t.Fatalf("connection received %d messages, want 1", len(writer.sent))
	}
	var msg struct {
		Type            string `json:"type"`
		ListingID       string `json:"listing_id"`
		BidID           string `json:"bid_id"`
		CurrentBid      int64  `json:"current_bid"`
		BidCount        int    `json:"bid_count"`
		HighBidderIsYou bool   `json:"high_bidder_is_you"`
		AcceptedAt      string `json:"accepted_at"`
	}
	if err := json.Unmarshal(writer.sent[0], &msg); err != nil {
		t.Fatalf("unmarshal received message: %v (%s)", err, writer.sent[0])
	}
	if msg.Type != "bid_accepted" || msg.ListingID != listing.ID || msg.CurrentBid != 21_000 || msg.BidCount != 1 {
		t.Errorf("message = %+v, want bid_accepted for %s at 21000/1", msg, listing.ID)
	}
	if msg.HighBidderIsYou {
		t.Error("high_bidder_is_you = true for a viewer session that didn't place this bid, want false")
	}
	if msg.BidID == "" || msg.AcceptedAt == "" {
		t.Errorf("message = %+v, want non-empty bid_id/accepted_at", msg)
	}
}

// TestWSHub_RealBuyNowReachesASubscribedConnectionAsListingEnded mirrors the
// above for the buy-now path, which the stream tailer publishes as
// listing_ended/bought_now rather than bid_accepted (guidelines/06-backend-architecture.md,
// "listing_ended push for natural time expiry") -- including proving the
// buyer's own subscribed connection gets this broadcast too, same as any
// other viewer, since a buy-now response is confirmed to the buyer over REST,
// not WS.
func TestWSHub_RealBuyNowReachesASubscribedConnectionAsListingEnded(t *testing.T) {
	ctx := context.Background()
	pool := newPoolAndMigrate(t)
	rdb := startRedis(t)

	listing := sampleListing("99999999-8888-8888-8888-888888888888", "VINWSBUY1")
	listing.AuctionStart = time.Now().Add(-time.Hour)
	listing.AuctionDuration = 24 * time.Hour
	buyNowPrice := int64(50_000)
	listing.BuyNowPrice = &buyNowPrice
	if _, err := pgstore.InsertNewListings(ctx, pool, []domain.Listing{listing}); err != nil {
		t.Fatalf("InsertNewListings: %v", err)
	}
	fromPostgres, err := pgstore.NewListingReader(pool).Get(ctx, listing.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := redisstore.PrimeListingState(ctx, rdb, fromPostgres); err != nil {
		t.Fatalf("PrimeListingState: %v", err)
	}

	broadcaster := inmemory.NewBroadcaster()
	writer := &jsonCapturingWriter{}
	conn := ws.NewConnection(writer, "session-buyer")
	broadcaster.Subscribe(conn, listing.ID)

	bidStore := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	if _, err := bidStore.BuyNow(ctx, listing.ID, "session-buyer"); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}

	tailer := redisstore.NewStreamTailer(rdb, pool, broadcaster)
	if err := tailer.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(writer.sent) != 1 {
		t.Fatalf("connection received %d messages, want 1", len(writer.sent))
	}
	var msg struct {
		Type       string `json:"type"`
		ListingID  string `json:"listing_id"`
		Reason     string `json:"reason"`
		FinalPrice int64  `json:"final_price"`
	}
	if err := json.Unmarshal(writer.sent[0], &msg); err != nil {
		t.Fatalf("unmarshal received message: %v (%s)", err, writer.sent[0])
	}
	if msg.Type != "listing_ended" || msg.Reason != "bought_now" || msg.FinalPrice != 50_000 || msg.ListingID != listing.ID {
		t.Errorf("message = %+v, want listing_ended/bought_now at 50000 for %s", msg, listing.ID)
	}
}
