// Package handlers holds the actual HTTP handler implementations
// (guidelines/06-backend-architecture.md).
package handlers

import (
	"net/http"
	"time"

	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// Listings serves the read endpoints backed by listings.Reader -- a port
// interface, not a concrete pgstore type, so this is testable with a fake
// (guidelines/06-backend-architecture.md).
type Listings struct {
	reader listings.Reader
	now    func() time.Time // injectable for deterministic status-computation tests
}

func NewListings(reader listings.Reader) *Listings {
	return &Listings{reader: reader, now: time.Now}
}

// List handles GET /v1/listings. Deliberately unpaginated for now -- cursor
// pagination is a dedicated later commit, kept separate so basic filtering is
// revertable independently of the more intricate cursor logic
// (guidelines/06-backend-architecture.md).
func (h *Listings) List(w http.ResponseWriter, r *http.Request) {
	filter := listings.Filter{
		Status: r.URL.Query().Get("status"),
		Make:   r.URL.Query().Get("make"),
		Search: r.URL.Query().Get("q"),
	}

	rows, err := h.reader.ListFiltered(r.Context(), filter)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error",
			"Something went wrong.", nil, middleware.RequestIDFromContext(r.Context()))
		return
	}

	now := h.now()
	summaries := make([]dto.ListingSummary, len(rows))
	for i, l := range rows {
		summaries[i] = dto.NewListingSummary(l, now)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"data": summaries})
}

// Facets handles GET /v1/listings/facets.
func (h *Listings) Facets(w http.ResponseWriter, r *http.Request) {
	makes, err := h.reader.DistinctMakes(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error",
			"Something went wrong.", nil, middleware.RequestIDFromContext(r.Context()))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dto.Facets{Makes: makes})
}
