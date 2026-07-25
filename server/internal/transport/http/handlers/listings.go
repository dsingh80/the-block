// Package handlers holds the actual HTTP handler implementations
// (guidelines/06-backend-architecture.md).
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
	"github.com/dsingh80/the-block/server/internal/usecase/audit"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

const defaultPageSize = 24

// defaultBidHistoryLimit caps how many of a listing's recorded bids GET
// /v1/listings/{id}/bids returns. Not cursor-paginated like the listings list
// (guidelines/06-backend-architecture.md, "Pagination"): a single listing's
// real recorded history is bounded by how many bids actually land against it
// during this project's lifetime, nowhere near the catalog-wide scale that
// motivated cursor pagination there. Revisit if that assumption ever breaks.
const defaultBidHistoryLimit = 200

var validSortModes = map[string]listings.SortMode{
	"ending":     listings.SortEnding,
	"price-low":  listings.SortPriceLow,
	"price-high": listings.SortPriceHigh,
	"year":       listings.SortYear,
}

// Listings serves the read endpoints backed by listings.Reader, audit.Reader,
// and bidding.ViewerLookup -- port interfaces, not concrete pgstore/redisstore
// types, so this is testable with fakes (guidelines/06-backend-architecture.md).
type Listings struct {
	reader       listings.Reader
	bidReader    audit.Reader
	viewerLookup bidding.ViewerLookup
	now          func() time.Time // injectable for deterministic status-computation tests
}

func NewListings(reader listings.Reader, bidReader audit.Reader, viewerLookup bidding.ViewerLookup) *Listings {
	return &Listings{reader: reader, bidReader: bidReader, viewerLookup: viewerLookup, now: time.Now}
}

// viewersFor computes each listing's Viewer, then corrects any Postgres-stale
// false "outbid" via one batched Redis lookup covering just the listings that
// actually need it -- has_bid true but not the Postgres-recorded high bidder,
// which is what a session's own just-accepted bid looks like until the stream
// tailer's next tick drains it (guidelines/06-backend-architecture.md,
// "Draining vs. broadcasting"). See domain.Viewer.ReconcileHighBidder for why
// domain.ComputeViewer alone can't tell that apart from a genuine outbid.
// The common case -- nothing ambiguous on this page -- costs no extra lookup.
func (h *Listings) viewersFor(ctx context.Context, ls []domain.Listing, sessionToken string, bidListingIDs map[string]struct{}) ([]domain.Viewer, error) {
	viewers := make([]domain.Viewer, len(ls))
	var ambiguous []string
	for i, l := range ls {
		viewers[i] = domain.ComputeViewer(l, sessionToken, bidListingIDs)
		if viewers[i].HasBid && !viewers[i].IsHighBidder {
			ambiguous = append(ambiguous, l.ID)
		}
	}
	if len(ambiguous) == 0 {
		return viewers, nil
	}

	live, err := h.viewerLookup.HighBidderSessions(ctx, ambiguous)
	if err != nil {
		return nil, err
	}
	for i, l := range ls {
		viewers[i] = viewers[i].ReconcileHighBidder(live[l.ID], sessionToken)
	}
	return viewers, nil
}

// sessionTokenOf reads the resolved session set by middleware.Session -- always
// present in production (the middleware runs on every route), empty in a
// handler test that doesn't wire it, which sessionTokenOf/domain.ComputeViewer
// both treat as "no session," not a panic.
func sessionTokenOf(r *http.Request) string {
	sess, _ := middleware.SessionFromContext(r.Context())
	return sess.Token
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

	sessionToken := sessionTokenOf(r)
	bidListingIDs, err := h.viewerLookup.BidListingIDs(r.Context(), sessionToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	viewers, err := h.viewersFor(r.Context(), page.Items, sessionToken, bidListingIDs)
	if err != nil {
		writeError(w, r, err)
		return
	}

	now := h.now()
	summaries := make([]dto.ListingSummary, len(page.Items))
	for i, l := range page.Items {
		summaries[i] = dto.NewListingSummary(l, now, viewers[i])
	}
	httputil.WriteJSON(w, http.StatusOK, dto.ListingsPage{
		Data: summaries,
		PageInfo: dto.PageInfo{
			HasNextPage: page.HasNextPage, HasPrevPage: page.HasPrevPage,
			StartCursor: page.StartCursor, EndCursor: page.EndCursor,
		},
	})
}

// Get handles GET /v1/listings/{id}. current_price/bid_count still come from
// Postgres only, no Redis overlay for zero-lag price (guidelines/06-backend-architecture.md,
// "GET /v1/listings/{id} freshness") -- anyone actively viewing gets true-live
// price updates over the WebSocket instead, and this stays consistent with the
// list endpoint plus portable to a different transport later (D3). viewer is a
// narrower exception: see viewersFor.
func (h *Listings) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseListingID(w, r)
	if !ok {
		return
	}

	l, err := h.reader.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	sessionToken := sessionTokenOf(r)
	bidListingIDs, err := h.viewerLookup.BidListingIDs(r.Context(), sessionToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	viewers, err := h.viewersFor(r.Context(), []domain.Listing{l}, sessionToken, bidListingIDs)
	if err != nil {
		writeError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, dto.NewListingSummary(l, h.now(), viewers[0]))
}

// BidHistory handles GET /v1/listings/{id}/bids -- the anonymized audit trail
// (guidelines/06-backend-architecture.md, "Bid-history anonymization").
func (h *Listings) BidHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseListingID(w, r)
	if !ok {
		return
	}

	if _, err := h.reader.Get(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}

	bids, err := h.bidReader.ListForListing(r.Context(), id, defaultBidHistoryLimit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, dto.NewBidHistory(bids, sessionTokenOf(r)))
}

// parseListingID validates the {id} path value is a well-formed UUID before it
// ever reaches a store -- boundary validation (this is user input), not a
// scenario internal code needs to guard against elsewhere. A malformed id is
// answered as 404, the same as a well-formed one that doesn't exist: from a
// client's perspective both mean "this listing isn't there," and no legitimate
// client following a server-provided id could ever produce a malformed one.
func parseListingID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, r, domain.ErrNotFound)
		return "", false
	}
	return id, true
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
