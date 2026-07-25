package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

const (
	// pingInterval is how often the server pings an idle connection.
	pingInterval = 30 * time.Second
	// pongWait is how long a connection has to answer a ping (or send any
	// other frame) before it's considered dead -- a generous multiple of
	// pingInterval so one delayed pong doesn't kill a healthy connection.
	pongWait = 60 * time.Second
)

// Hub upgrades HTTP requests to WebSocket connections and wires each one to a
// realtime.Broadcaster (guidelines/06-backend-architecture.md, "WebSocket protocol").
type Hub struct {
	broadcaster    realtime.Broadcaster
	allowedOrigins map[string]struct{}
	upgrader       websocket.Upgrader

	// pingInterval/pongWait default to the package constants above; only ever
	// overridden by this package's own tests (unexported, no setter) to
	// exercise a real ping/pong tick without an actual 30s wait.
	pingInterval time.Duration
	pongWait     time.Duration
}

// NewHub builds a Hub. allowedOrigins is the WS handshake's Origin allow-list
// (guidelines/06-backend-architecture.md): SameSite cookie attributes don't
// reliably cover the WS handshake the way they cover ordinary requests, so
// Origin is checked explicitly as a second layer.
func NewHub(broadcaster realtime.Broadcaster, allowedOrigins []string) *Hub {
	h := &Hub{
		broadcaster:    broadcaster,
		allowedOrigins: make(map[string]struct{}, len(allowedOrigins)),
		pingInterval:   pingInterval,
		pongWait:       pongWait,
	}
	for _, o := range allowedOrigins {
		h.allowedOrigins[o] = struct{}{}
	}
	h.upgrader = websocket.Upgrader{CheckOrigin: h.checkOrigin}
	return h
}

func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // no Origin header at all -- a non-browser client (curl, a test), not a cross-site page
	}
	_, ok := h.allowedOrigins[origin]
	return ok
}

// Upgrade handles GET /v1/ws. Registered behind the same middleware chain as
// every other /v1/* route, so the session is already resolved by the time
// this runs (guidelines/06-backend-architecture.md).
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request) {
	sess, _ := middleware.SessionFromContext(r.Context())

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote its own HTTP error response on failure
	}
	defer conn.Close()

	c := NewConnection(conn, sess.Token)
	h.serve(conn, c)
}

func (h *Hub) serve(conn *websocket.Conn, c *Connection) {
	defer h.broadcaster.Unsubscribe(c, c.SubscribedListingIDs()...)

	_ = conn.SetReadDeadline(time.Now().Add(h.pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(h.pongWait))
	})

	done := make(chan struct{})
	defer close(done)
	go h.pingLoop(conn, done)

	for {
		var msg clientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return // closed, or a real read error -- either way this connection is done
		}
		h.handleMessage(c, msg)
	}
}

func (h *Hub) pingLoop(conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) handleMessage(c *Connection, msg clientMessage) {
	switch msg.Type {
	case "subscribe":
		subscribed, capped := c.Subscribe(msg.ListingIDs)
		h.broadcaster.Subscribe(c, subscribed...)
		if err := c.SendAck(subscribed); err != nil {
			return
		}
		if capped {
			_ = c.SendError("subscription_cap_exceeded", "Too many subscribed listings.")
		}
	case "unsubscribe":
		c.Unsubscribe(msg.ListingIDs)
		h.broadcaster.Unsubscribe(c, msg.ListingIDs...)
	default:
		_ = c.SendError("invalid_message", "Unrecognized message type.")
	}
}
