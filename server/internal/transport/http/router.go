// Package http is the HTTP transport: routing, middleware, and handlers
// (guidelines/06-backend-architecture.md).
package http

import (
	"net/http"

	"github.com/dsingh80/the-block/server/internal/transport/http/handlers"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/usecase/audit"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
	"github.com/dsingh80/the-block/server/internal/usecase/sessions"
)

// NewRouter wires every registered route behind the full middleware chain.
// Session and CSRF apply globally -- CSRF only ever blocks state-changing
// methods, so wiring it now is harmless even before any such route exists,
// and every request (read-only included) getting a resolved session is what
// the rest of this API assumes (guidelines/06-backend-architecture.md).
func NewRouter(listingReader listings.Reader, bidReader audit.Reader, viewerLookup bidding.ViewerLookup, sessionStore sessions.Store) http.Handler {
	mux := http.NewServeMux()

	listingsHandler := handlers.NewListings(listingReader, bidReader, viewerLookup)
	mux.HandleFunc("GET /v1/listings", listingsHandler.List)
	mux.HandleFunc("GET /v1/listings/facets", listingsHandler.Facets)
	mux.HandleFunc("GET /v1/listings/{id}", listingsHandler.Get)
	mux.HandleFunc("GET /v1/listings/{id}/bids", listingsHandler.BidHistory)

	return Chain(mux,
		middleware.RequestID,
		middleware.Recover,
		middleware.AccessLog,
		middleware.Session(sessionStore),
		middleware.CSRF,
	)
}

// Chain applies middleware around h in the given order, outermost first --
// Chain(h, A, B) means a request passes through A, then B, then h.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
