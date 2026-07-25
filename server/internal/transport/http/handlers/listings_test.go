package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// sampleListingUUID is a well-formed UUID for the tests below that go through
// Get/BidHistory -- unlike sampleDomainListing's "abc-123", these handlers
// validate the {id} path value as a real UUID before touching any store.
const sampleListingUUID = "11111111-1111-1111-1111-111111111111"

func sampleDomainListingWithUUID() domain.Listing {
	l := sampleDomainListing()
	l.ID = sampleListingUUID
	return l
}

// fixedSessionStore is a trivial sessions.Store fake that always resolves to
// the same session, regardless of the inbound cookie -- lets a handler test
// exercise a real resolved session (via the real middleware.Session, not a
// hand-rolled context value) instead of every request reading back token="".
type fixedSessionStore struct {
	token     string
	createdAt time.Time
}

func (f fixedSessionStore) Touch(_ context.Context, _ string) (domain.Session, bool, error) {
	return domain.Session{Token: f.token, CreatedAt: f.createdAt}, false, nil
}

func withFixedSession(token string, h http.HandlerFunc) http.Handler {
	return middleware.Session(fixedSessionStore{token: token})(h)
}

func withFixedSessionAt(token string, createdAt time.Time, h http.HandlerFunc) http.Handler {
	return middleware.Session(fixedSessionStore{token: token, createdAt: createdAt})(h)
}

// fakeReader is a spy: it records the last PageRequest it was called with (so
// tests can assert the handler wired query params correctly) and returns a
// fixed dataset -- SQL filter/pagination semantics themselves are covered by
// the pgstore integration tests, not re-tested here.
type fakeReader struct {
	all         []domain.Listing
	makes       []string
	err         error
	page        listings.Page // returned verbatim by ListPage when set
	lastPageReq listings.PageRequest
	getCalls    int
}

func (f *fakeReader) Get(_ context.Context, id string) (domain.Listing, error) {
	f.getCalls++
	for _, l := range f.all {
		if l.ID == id {
			return l, nil
		}
	}
	return domain.Listing{}, domain.ErrNotFound
}

// fakeBidReader is a spy for audit.Reader.
type fakeBidReader struct {
	bids   []domain.Bid
	err    error
	calls  int
	limits []int
}

func (f *fakeBidReader) ListForListing(_ context.Context, _ string, limit int) ([]domain.Bid, error) {
	f.calls++
	f.limits = append(f.limits, limit)
	if f.err != nil {
		return nil, f.err
	}
	return f.bids, nil
}

// fakeViewerLookup is a spy for bidding.ViewerLookup.
type fakeViewerLookup struct {
	ids   map[string]struct{}
	err   error
	calls int

	highBidders       map[string]string // listing id -> live Redis high_bidder_session
	highBidderErr     error
	highBidderCalls   int
	lastHighBidderIDs []string
}

func (f *fakeViewerLookup) BidListingIDs(_ context.Context, _ string) (map[string]struct{}, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.ids == nil {
		return map[string]struct{}{}, nil
	}
	return f.ids, nil
}

func (f *fakeViewerLookup) HighBidderSessions(_ context.Context, listingIDs []string) (map[string]string, error) {
	f.highBidderCalls++
	f.lastHighBidderIDs = listingIDs
	if f.highBidderErr != nil {
		return nil, f.highBidderErr
	}
	if f.highBidders == nil {
		return map[string]string{}, nil
	}
	return f.highBidders, nil
}

func (f *fakeReader) ListPage(_ context.Context, req listings.PageRequest) (listings.Page, error) {
	f.lastPageReq = req
	if f.err != nil {
		return listings.Page{}, f.err
	}
	if f.page.Items != nil || f.page.HasNextPage || f.page.HasPrevPage {
		return f.page, nil
	}
	return listings.Page{Items: f.all}, nil
}

func (f *fakeReader) DistinctMakes(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.makes, nil
}

func sampleDomainListing() domain.Listing {
	reserve := int64(29_000)
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return domain.Listing{
		ID: "abc-123", VIN: "VIN1", Year: 2025, Make: "Mazda", Model: "CX-5",
		AuctionStart: start, AuctionEnd: start.Add(24 * time.Hour),
		StartingBid: 20_500, ReservePrice: &reserve, CurrentPrice: 20_500,
		DamageNotes: []string{}, Images: []string{},
	}
}

func TestListingsList_WiresQueryParamsIntoPageRequest(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?status=active&make=Mazda&q=cx-5&sort=price-low&first=10&after=some-cursor", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	want := listings.PageRequest{
		Filter: listings.Filter{Status: "active", Make: "Mazda", Search: "cx-5"},
		Sort:   listings.SortPriceLow, First: 10, After: "some-cursor", Last: 24,
	}
	if reader.lastPageReq != want {
		t.Errorf("ListPage called with %+v, want %+v", reader.lastPageReq, want)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListingsList_DefaultsSortToEndingAndPageSizeTo24(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if reader.lastPageReq.Sort != listings.SortEnding {
		t.Errorf("Sort = %q, want the default %q", reader.lastPageReq.Sort, listings.SortEnding)
	}
	if reader.lastPageReq.First != defaultPageSize || reader.lastPageReq.Last != defaultPageSize {
		t.Errorf("First/Last = %d/%d, want both defaulted to %d", reader.lastPageReq.First, reader.lastPageReq.Last, defaultPageSize)
	}
}

func TestListingsList_RejectsAnUnknownSort(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?sort=cheapest-first", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unrecognized sort value", rec.Code)
	}
}

func TestListingsList_SerializesDataPageInfoAndNeverIncludesReservePrice(t *testing.T) {
	reader := &fakeReader{page: listings.Page{
		Items:       []domain.Listing{sampleDomainListing()},
		HasNextPage: true, StartCursor: "start-cur", EndCursor: "end-cur",
	}}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if strings.Contains(rec.Body.String(), "reserve") {
		t.Errorf("response body mentions reserve_price, want it entirely absent: %s", rec.Body.String())
	}

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Make string `json:"make"`
		} `json:"data"`
		PageInfo struct {
			HasNextPage bool   `json:"has_next_page"`
			HasPrevPage bool   `json:"has_previous_page"`
			StartCursor string `json:"start_cursor"`
			EndCursor   string `json:"end_cursor"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Data) != 1 || body.Data[0].ID != "abc-123" || body.Data[0].Make != "Mazda" {
		t.Errorf("data = %+v, want exactly the one fake listing", body.Data)
	}
	if !body.PageInfo.HasNextPage || body.PageInfo.HasPrevPage || body.PageInfo.StartCursor != "start-cur" || body.PageInfo.EndCursor != "end-cur" {
		t.Errorf("page_info = %+v, want it to match the reader's returned Page", body.PageInfo)
	}
}

func TestListingsList_ReaderErrorReturns500(t *testing.T) {
	reader := &fakeReader{err: errors.New("boom")}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListingsList_ViewerLookupErrorReturns500(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	viewerLookup := &fakeViewerLookup{err: errors.New("redis down")}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when BidListingIDs errors", rec.Code)
	}
}

func TestListingsList_InvalidCursorMapsTo400(t *testing.T) {
	reader := &fakeReader{err: domain.ErrInvalidCursor}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?after=garbage", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for domain.ErrInvalidCursor", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Error.Code != "invalid_cursor" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "invalid_cursor")
	}
}

func TestListingsFacets_ReaderErrorReturns500(t *testing.T) {
	reader := &fakeReader{err: errors.New("boom")}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/facets", nil)
	rec := httptest.NewRecorder()
	h.Facets(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when DistinctMakes errors", rec.Code)
	}
}

func TestListingsFacets_ReturnsDistinctMakes(t *testing.T) {
	reader := &fakeReader{makes: []string{"Chevrolet", "Mazda", "Toyota"}}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/facets", nil)
	rec := httptest.NewRecorder()
	h.Facets(rec, req)

	var body struct {
		Makes []string `json:"makes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Makes) != 3 || body.Makes[1] != "Mazda" {
		t.Errorf("makes = %v, want [Chevrolet, Mazda, Toyota]", body.Makes)
	}
}

func TestListingsGet_ReturnsListingWithViewer(t *testing.T) {
	const viewerToken = "session-viewer"
	highBidder := viewerToken
	listing := sampleDomainListingWithUUID()
	listing.HighBidderSessionID = &highBidder

	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{ids: map[string]struct{}{sampleListingUUID: {}}}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.Get).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if reader.getCalls != 1 {
		t.Errorf("reader.Get called %d times, want exactly 1", reader.getCalls)
	}
	if viewerLookup.calls != 1 {
		t.Errorf("viewerLookup.BidListingIDs called %d times, want exactly 1 -- no per-row/N+1 lookups", viewerLookup.calls)
	}

	var body struct {
		ID     string     `json:"id"`
		Viewer dto.Viewer `json:"viewer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body.ID != sampleListingUUID {
		t.Errorf("id = %q, want %q", body.ID, sampleListingUUID)
	}
	want := dto.Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}
	if body.Viewer != want {
		t.Errorf("viewer = %+v, want %+v", body.Viewer, want)
	}
}

// TestListingsGet_ReconcilesPostgresStaleOutbid is the regression test for the
// bug this reconciliation exists to fix: a session whose own bid was just
// accepted has has_bid=true (Redis, instant) immediately, but the listing's
// Postgres-derived HighBidderSessionID can still name the *previous* high
// bidder until the stream tailer's next tick drains it -- ComputeViewer alone
// can't tell that apart from a genuine outbid, so without reconciliation this
// session would see "You Have Been Outbid" on its own winning bid.
func TestListingsGet_ReconcilesPostgresStaleOutbid(t *testing.T) {
	const viewerToken = "session-viewer"
	staleHighBidder := "session-previous-bidder"
	listing := sampleDomainListingWithUUID()
	listing.HighBidderSessionID = &staleHighBidder // Postgres hasn't drained this session's new bid yet

	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{
		ids:         map[string]struct{}{sampleListingUUID: {}},        // has_bid: Redis already reflects it
		highBidders: map[string]string{sampleListingUUID: viewerToken}, // Redis: this session IS the live high bidder
	}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.Get).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if viewerLookup.highBidderCalls != 1 {
		t.Errorf("HighBidderSessions called %d times, want exactly 1", viewerLookup.highBidderCalls)
	}
	if got := viewerLookup.lastHighBidderIDs; len(got) != 1 || got[0] != sampleListingUUID {
		t.Errorf("HighBidderSessions called with %v, want exactly [%q]", got, sampleListingUUID)
	}

	var body struct {
		Viewer dto.Viewer `json:"viewer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	want := dto.Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}
	if body.Viewer != want {
		t.Errorf("viewer = %+v, want %+v -- Postgres staleness must not surface as this session's own bid being outbid", body.Viewer, want)
	}
}

// TestListingsGet_GenuineOutbidStaysOutbid proves reconciliation doesn't
// overcorrect: when Redis's live high bidder agrees with Postgres that
// someone else is winning, the viewer must still read outbid.
func TestListingsGet_GenuineOutbidStaysOutbid(t *testing.T) {
	const viewerToken = "session-viewer"
	rival := "session-rival"
	listing := sampleDomainListingWithUUID()
	listing.HighBidderSessionID = &rival

	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{
		ids:         map[string]struct{}{sampleListingUUID: {}},
		highBidders: map[string]string{sampleListingUUID: rival}, // Redis agrees: rival is really winning
	}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.Get).ServeHTTP(rec, req)

	var body struct {
		Viewer dto.Viewer `json:"viewer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	want := dto.Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true}
	if body.Viewer != want {
		t.Errorf("viewer = %+v, want %+v -- a real outbid must survive reconciliation", body.Viewer, want)
	}
}

// TestListingsGet_NeverAmbiguousSkipsHighBidderLookup guards the cheap path:
// a session that's already the high bidder (or never bid at all) has nothing
// for Redis to disambiguate, so HighBidderSessions must not be called --
// otherwise every single listing view would pay for a lookup that's almost
// always pointless.
func TestListingsGet_NeverAmbiguousSkipsHighBidderLookup(t *testing.T) {
	const viewerToken = "session-viewer"
	highBidder := viewerToken
	listing := sampleDomainListingWithUUID()
	listing.HighBidderSessionID = &highBidder // already the recorded high bidder

	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{ids: map[string]struct{}{sampleListingUUID: {}}}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.Get).ServeHTTP(rec, req)

	if viewerLookup.highBidderCalls != 0 {
		t.Errorf("HighBidderSessions called %d times, want 0 -- already the high bidder is never ambiguous", viewerLookup.highBidderCalls)
	}
}

func TestListingsGet_HighBidderSessionsErrorReturns500(t *testing.T) {
	const viewerToken = "session-viewer"
	staleHighBidder := "session-previous-bidder"
	listing := sampleDomainListingWithUUID()
	listing.HighBidderSessionID = &staleHighBidder

	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{
		ids:           map[string]struct{}{sampleListingUUID: {}},
		highBidderErr: errors.New("redis down"),
	}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.Get).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when HighBidderSessions errors", rec.Code)
	}
}

// TestListingsList_BatchesAmbiguousHighBidderLookupAcrossRows proves the List
// endpoint reconciles the same way as Get, and does it with one batched
// HighBidderSessions call across every ambiguous row on the page -- not one
// call per row.
func TestListingsList_BatchesAmbiguousHighBidderLookupAcrossRows(t *testing.T) {
	const viewerToken = "session-viewer"
	staleHighBidder := "session-previous-bidder"
	highBidder := viewerToken

	ambiguousA := sampleDomainListing()
	ambiguousA.ID = "listing-ambiguous-a"
	ambiguousA.HighBidderSessionID = &staleHighBidder

	ambiguousB := sampleDomainListing()
	ambiguousB.ID = "listing-ambiguous-b"
	ambiguousB.HighBidderSessionID = &staleHighBidder

	alreadyWinning := sampleDomainListing()
	alreadyWinning.ID = "listing-already-winning"
	alreadyWinning.HighBidderSessionID = &highBidder

	reader := &fakeReader{page: listings.Page{Items: []domain.Listing{ambiguousA, ambiguousB, alreadyWinning}}}
	viewerLookup := &fakeViewerLookup{
		ids: map[string]struct{}{ambiguousA.ID: {}, ambiguousB.ID: {}, alreadyWinning.ID: {}},
		highBidders: map[string]string{
			ambiguousA.ID: viewerToken, // Redis: actually mine, Postgres just hasn't drained yet
			// ambiguousB deliberately absent: Redis agrees no one's outbid-corrected it, stays outbid
		},
	}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	withFixedSession(viewerToken, h.List).ServeHTTP(rec, req)

	if viewerLookup.highBidderCalls != 1 {
		t.Fatalf("HighBidderSessions called %d times, want exactly 1 (batched, not per-row)", viewerLookup.highBidderCalls)
	}
	gotIDs := append([]string{}, viewerLookup.lastHighBidderIDs...)
	sort.Strings(gotIDs)
	wantIDs := []string{ambiguousA.ID, ambiguousB.ID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("HighBidderSessions called with %v, want exactly %v -- the already-winning row must not be included", gotIDs, wantIDs)
	}

	var body struct {
		Data []struct {
			ID     string     `json:"id"`
			Viewer dto.Viewer `json:"viewer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	viewerByID := make(map[string]dto.Viewer, len(body.Data))
	for _, row := range body.Data {
		viewerByID[row.ID] = row.Viewer
	}
	if got, want := viewerByID[ambiguousA.ID], (dto.Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}); got != want {
		t.Errorf("ambiguousA viewer = %+v, want %+v (reconciled to winning)", got, want)
	}
	if got, want := viewerByID[ambiguousB.ID], (dto.Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true}); got != want {
		t.Errorf("ambiguousB viewer = %+v, want %+v (genuine outbid, left alone)", got, want)
	}
	if got, want := viewerByID[alreadyWinning.ID], (dto.Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false}); got != want {
		t.Errorf("alreadyWinning viewer = %+v, want %+v (never ambiguous)", got, want)
	}
}

func TestListingsGet_ViewerLookupErrorReturns500(t *testing.T) {
	listing := sampleDomainListingWithUUID()
	reader := &fakeReader{all: []domain.Listing{listing}}
	viewerLookup := &fakeViewerLookup{err: errors.New("redis down")}
	h := NewListings(reader, &fakeBidReader{}, viewerLookup)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID, nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when BidListingIDs errors", rec.Code)
	}
}

func TestListingsGet_UnknownIdReturns404(t *testing.T) {
	reader := &fakeReader{}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	unknownID := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+unknownID, nil)
	req.SetPathValue("id", unknownID)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a well-formed but unknown id", rec.Code)
	}
}

func TestListingsGet_MalformedIdReturns404(t *testing.T) {
	reader := &fakeReader{}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a malformed id", rec.Code)
	}
	if reader.getCalls != 0 {
		t.Errorf("reader.Get called %d times, want 0 -- a malformed id must be rejected before touching any store", reader.getCalls)
	}
}

func TestListingsBidHistory_ReturnsAnonymizedHistoryNewestFirst(t *testing.T) {
	listing := sampleDomainListingWithUUID()
	reader := &fakeReader{all: []domain.Listing{listing}}
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	bidReader := &fakeBidReader{bids: []domain.Bid{
		{ID: "bid-1", ListingID: sampleListingUUID, SessionID: "session-a", Type: domain.BidTypeBid, Amount: 21_000, BidCountAfter: 1, AcceptedAt: t0},
		{ID: "bid-2", ListingID: sampleListingUUID, SessionID: "session-b", Type: domain.BidTypeBid, Amount: 21_500, BidCountAfter: 2, AcceptedAt: t0.Add(time.Minute)},
	}}
	h := NewListings(reader, bidReader, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID+"/bids", nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.BidHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if reader.getCalls != 1 {
		t.Errorf("reader.Get (existence check) called %d times, want exactly 1", reader.getCalls)
	}
	if got := bidReader.limits; len(got) != 1 || got[0] != defaultBidHistoryLimit {
		t.Errorf("bidReader.ListForListing called with limits %v, want exactly [%d]", got, defaultBidHistoryLimit)
	}

	var body dto.BidHistory
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Data) != 2 || body.Data[0].Handle != "Bidder 2" || body.Data[1].Handle != "Bidder 1" {
		t.Fatalf("data handles = %+v, want [\"Bidder 2\", \"Bidder 1\"] (newest first, numbered by first appearance)", body.Data)
	}
}

func TestListingsBidHistory_BidReaderErrorReturns500(t *testing.T) {
	listing := sampleDomainListingWithUUID()
	reader := &fakeReader{all: []domain.Listing{listing}}
	bidReader := &fakeBidReader{err: errors.New("pg down")}
	h := NewListings(reader, bidReader, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+sampleListingUUID+"/bids", nil)
	req.SetPathValue("id", sampleListingUUID)
	rec := httptest.NewRecorder()
	h.BidHistory(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when ListForListing errors", rec.Code)
	}
}

func TestListingsBidHistory_UnknownIdReturns404WithoutQueryingBids(t *testing.T) {
	reader := &fakeReader{}
	bidReader := &fakeBidReader{}
	h := NewListings(reader, bidReader, &fakeViewerLookup{})

	unknownID := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest(http.MethodGet, "/v1/listings/"+unknownID+"/bids", nil)
	req.SetPathValue("id", unknownID)
	rec := httptest.NewRecorder()
	h.BidHistory(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a well-formed but unknown id", rec.Code)
	}
	if bidReader.calls != 0 {
		t.Errorf("bidReader.ListForListing called %d times, want 0 -- no point querying bids for a listing that doesn't exist", bidReader.calls)
	}
}

func TestListingsBidHistory_MalformedIdReturns404(t *testing.T) {
	reader := &fakeReader{}
	h := NewListings(reader, &fakeBidReader{}, &fakeViewerLookup{})

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/not-a-uuid/bids", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.BidHistory(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a malformed id", rec.Code)
	}
}

func TestParsePageSize(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"empty falls back to the default", "", defaultPageSize},
		{"a valid positive integer is used as-is", "10", 10},
		{"non-numeric input falls back to the default", "abc", defaultPageSize},
		{"zero falls back to the default", "0", defaultPageSize},
		{"negative falls back to the default", "-5", defaultPageSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePageSize(tc.raw); got != tc.want {
				t.Errorf("parsePageSize(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}
