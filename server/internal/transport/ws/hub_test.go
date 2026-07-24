package ws

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/inmemory"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// fixedSessionStore is a trivial sessions.Store fake -- the same pattern used
// in internal/transport/http/handlers, reimplemented here since Go test
// doubles aren't exported across packages.
type fixedSessionStore struct{ token string }

func (f fixedSessionStore) Touch(_ context.Context, _ string) (domain.Session, bool, error) {
	return domain.Session{Token: f.token}, false, nil
}

// newTestServer wraps hub.Upgrade in exactly the one piece of middleware it
// actually depends on (a resolved session in context) -- a real net/http
// server is required here full stop, since gorilla/websocket's Upgrade needs
// a genuinely hijackable connection that neither httptest.NewRecorder() nor a
// direct ServeHTTP call can provide.
func newTestServer(t *testing.T, hub *Hub, sessionToken string) *httptest.Server {
	t.Helper()
	handler := middleware.Session(fixedSessionStore{token: sessionToken})(http.HandlerFunc(hub.Upgrade))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func dial(t *testing.T, server *httptest.Server, origin string) *gorillaws.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	conn, resp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := ""
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("Dial: %v (response status: %s)", err, status)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestHub_UpgradeAndSubscribe_ReturnsAck(t *testing.T) {
	hub := NewHub(inmemory.NewBroadcaster(), nil) // no allow-list -- a Dial from this test sends no Origin header at all
	conn := dial(t, newTestServer(t, hub, "session-a"), "")

	if err := conn.WriteJSON(clientMessage{Type: "subscribe", ListingIDs: []string{"listing-1", "listing-2"}}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ack ackMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if ack.Type != "ack" || len(ack.Subscribed) != 2 {
		t.Errorf("ack = %+v, want type=ack with 2 subscribed ids", ack)
	}
}

func TestHub_SubscriptionCapEnforced(t *testing.T) {
	hub := NewHub(inmemory.NewBroadcaster(), nil)
	conn := dial(t, newTestServer(t, hub, "session-a"), "")

	ids := make([]string, MaxSubscriptionsPerConnection+10)
	for i := range ids {
		ids[i] = fmt.Sprintf("listing-%d", i)
	}
	if err := conn.WriteJSON(clientMessage{Type: "subscribe", ListingIDs: ids}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ack ackMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("ReadJSON(ack): %v", err)
	}
	if len(ack.Subscribed) != MaxSubscriptionsPerConnection {
		t.Errorf("ack.Subscribed = %d ids, want exactly the cap (%d)", len(ack.Subscribed), MaxSubscriptionsPerConnection)
	}

	var errMsg errorMessage
	if err := conn.ReadJSON(&errMsg); err != nil {
		t.Fatalf("ReadJSON(error): %v", err)
	}
	if errMsg.Type != "error" || errMsg.Code != "subscription_cap_exceeded" {
		t.Errorf("error message = %+v, want code=subscription_cap_exceeded", errMsg)
	}
}

func TestHub_OriginAllowList(t *testing.T) {
	hub := NewHub(inmemory.NewBroadcaster(), []string{"https://the-block.example"})
	server := newTestServer(t, hub, "session-a")

	t.Run("an allowed origin can connect", func(t *testing.T) {
		dial(t, server, "https://the-block.example")
	})

	t.Run("a disallowed origin is rejected", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		header := http.Header{}
		header.Set("Origin", "https://evil.example")
		_, resp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
		if err == nil {
			t.Fatal("Dial succeeded, want it rejected for a disallowed Origin")
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			got := 0
			if resp != nil {
				got = resp.StatusCode
			}
			t.Errorf("status = %d, want 403", got)
		}
	})
}

// TestHub_PublishedEventReachesSubscribedConnection is deliberately NOT the
// same claim as commit 27's end-to-end test (a real bid through the use-case
// layer reaching a subscriber) -- that wiring doesn't exist yet. This proves
// the piece that belongs to THIS commit: once subscribed via the real
// protocol, a Connection registered with a real Broadcaster receives and
// correctly translates an event, with high_bidder_is_you computed against
// this specific connection's own session.
func TestHub_PublishedEventReachesSubscribedConnection(t *testing.T) {
	broadcaster := inmemory.NewBroadcaster()
	hub := NewHub(broadcaster, nil)
	conn := dial(t, newTestServer(t, hub, "session-a"), "")

	if err := conn.WriteJSON(clientMessage{Type: "subscribe", ListingIDs: []string{"listing-1"}}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ack ackMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("ReadJSON(ack): %v", err)
	}

	broadcaster.Publish(context.Background(), realtime.Event{
		Type: "bid_accepted", ListingID: "listing-1", BidID: "bid-1",
		CurrentPrice: 21_500, BidCount: 3, HighBidderSessionID: "session-a",
		AcceptedAt: time.Now(),
	})

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var msg bidAcceptedMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("ReadJSON(bid_accepted): %v", err)
	}
	if msg.ListingID != "listing-1" || msg.CurrentBid != 21_500 || !msg.HighBidderIsYou {
		t.Errorf("message = %+v, want listing-1/21500/high_bidder_is_you=true", msg)
	}
}

func TestHub_UnsubscribeStopsFurtherDelivery(t *testing.T) {
	broadcaster := inmemory.NewBroadcaster()
	hub := NewHub(broadcaster, nil)
	conn := dial(t, newTestServer(t, hub, "session-a"), "")

	if err := conn.WriteJSON(clientMessage{Type: "subscribe", ListingIDs: []string{"listing-1"}}); err != nil {
		t.Fatalf("WriteJSON(subscribe): %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ack ackMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("ReadJSON(ack): %v", err)
	}

	if err := conn.WriteJSON(clientMessage{Type: "unsubscribe", ListingIDs: []string{"listing-1"}}); err != nil {
		t.Fatalf("WriteJSON(unsubscribe): %v", err)
	}
	// unsubscribe has no ack in the protocol -- give the server a moment to
	// actually process it before publishing (localhost, generous margin).
	time.Sleep(100 * time.Millisecond)

	broadcaster.Publish(context.Background(), realtime.Event{Type: "bid_accepted", ListingID: "listing-1"})

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var msg bidAcceptedMessage
	if err := conn.ReadJSON(&msg); err == nil {
		t.Errorf("received a message after unsubscribing: %+v, want a read timeout", msg)
	}
}
