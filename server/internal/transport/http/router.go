// Package http is the HTTP transport: routing, middleware, and handlers
// (guidelines/06-backend-architecture.md).
package http

import (
	"net/http"

	"github.com/dsingh80/the-block/server/internal/transport/http/handlers"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/ws"
	"github.com/dsingh80/the-block/server/internal/usecase/audit"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
	"github.com/dsingh80/the-block/server/internal/usecase/health"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
	"github.com/dsingh80/the-block/server/internal/usecase/sessions"
)

// NewRouter wires every registered route. /v1/* goes through the full
// middleware chain -- CSRF only ever blocks state-changing methods, so wiring
// it even for read-only routes is harmless, and every /v1/* request getting a
// resolved session is what the rest of this API assumes
// (guidelines/06-backend-architecture.md). /healthcheck is deliberately
// registered outside that chain: an orchestrator's liveness/readiness probe
// hitting it every few seconds shouldn't spin up a brand-new Redis session
// (and a Set-Cookie header on the response) on every single ping -- it still
// gets request id/panic-recovery/access-log, just not Session or CSRF.
func NewRouter(
	listingReader listings.Reader,
	bidReader audit.Reader,
	viewerLookup bidding.ViewerLookup,
	bidStore bidding.Store,
	rateLimiter bidding.RateLimiter,
	sessionStore sessions.Store,
	redisPinger health.Pinger,
	postgresPinger health.Pinger,
	broadcaster realtime.Broadcaster,
	wsAllowedOrigins []string,
) http.Handler {
	api := http.NewServeMux()

	listingsHandler := handlers.NewListings(listingReader, bidReader, viewerLookup)
	api.HandleFunc("GET /v1/listings", listingsHandler.List)
	api.HandleFunc("GET /v1/listings/facets", listingsHandler.Facets)
	api.HandleFunc("GET /v1/listings/{id}", listingsHandler.Get)
	api.HandleFunc("GET /v1/listings/{id}/bids", listingsHandler.BidHistory)

	bidsHandler := handlers.NewBids(bidStore, rateLimiter)
	api.HandleFunc("POST /v1/listings/{id}/bids", bidsHandler.Place)
	api.HandleFunc("POST /v1/listings/{id}/buy-now", bidsHandler.BuyNow)

	api.HandleFunc("GET /v1/session", handlers.SessionInfo)

	wsHub := ws.NewHub(broadcaster, wsAllowedOrigins)
	api.HandleFunc("GET /v1/ws", wsHub.Upgrade)

	root := http.NewServeMux()
	root.Handle("/", Chain(api,
		middleware.RequestID,
		middleware.Recover,
		middleware.AccessLog,
		middleware.Session(sessionStore),
		middleware.CSRF,
	))

	healthHandler := handlers.NewHealth(redisPinger, postgresPinger)
	root.Handle("GET /healthcheck", Chain(http.HandlerFunc(healthHandler.Check),
		middleware.RequestID,
		middleware.Recover,
		middleware.AccessLog,
	))

	return root
}

// Chain applies middleware around h in the given order, outermost first --
// Chain(h, A, B) means a request passes through A, then B, then h.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
