// Package handlers holds the actual HTTP handler implementations
// (guidelines/06-backend-architecture.md).
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

const defaultPageSize = 24

var validSortModes = map[string]listings.SortMode{
	"ending":     listings.SortEnding,
	"price-low":  listings.SortPriceLow,
	"price-high": listings.SortPriceHigh,
	"year":       listings.SortYear,
}

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

// List handles GET /v1/listings: cursor-paginated (guidelines/06-backend-architecture.md,
// "Cursor pagination"). Query params: status/make/q (filter), sort (default
// "ending"), first+after (forward) or last+before (backward), page size
// defaulting to 24 either way.
func (h *Listings) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	sortParam := q.Get("sort")
	if sortParam == "" {
		sortParam = "ending"
	}
	sort, ok := validSortModes[sortParam]
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_sort",
			"sort must be one of: ending, price-low, price-high, year.", nil, middleware.RequestIDFromContext(r.Context()))
		return
	}

	req := listings.PageRequest{
		Filter: listings.Filter{
			Status: q.Get("status"),
			Make:   q.Get("make"),
			Search: q.Get("q"),
		},
		Sort:   sort,
		First:  parsePageSize(q.Get("first")),
		After:  q.Get("after"),
		Last:   parsePageSize(q.Get("last")),
		Before: q.Get("before"),
	}

	page, err := h.reader.ListPage(r.Context(), req)
	if err != nil {
		writeError(w, r, err)
		return
	}

	now := h.now()
	summaries := make([]dto.ListingSummary, len(page.Items))
	for i, l := range page.Items {
		summaries[i] = dto.NewListingSummary(l, now)
	}
	httputil.WriteJSON(w, http.StatusOK, dto.ListingsPage{
		Data: summaries,
		PageInfo: dto.PageInfo{
			HasNextPage: page.HasNextPage, HasPrevPage: page.HasPrevPage,
			StartCursor: page.StartCursor, EndCursor: page.EndCursor,
		},
	})
}

func parsePageSize(raw string) int {
	if raw == "" {
		return defaultPageSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPageSize
	}
	return n
}

// Facets handles GET /v1/listings/facets.
func (h *Listings) Facets(w http.ResponseWriter, r *http.Request) {
	makes, err := h.reader.DistinctMakes(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dto.Facets{Makes: makes})
}
