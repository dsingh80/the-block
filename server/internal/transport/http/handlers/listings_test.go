package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

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
}

func (f *fakeReader) Get(_ context.Context, id string) (domain.Listing, error) {
	for _, l := range f.all {
		if l.ID == id {
			return l, nil
		}
	}
	return domain.Listing{}, domain.ErrNotFound
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
	h := NewListings(reader)

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
	h := NewListings(reader)

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
	h := NewListings(reader)

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
	h := NewListings(reader)

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
	h := NewListings(reader)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListingsList_InvalidCursorMapsTo400(t *testing.T) {
	reader := &fakeReader{err: domain.ErrInvalidCursor}
	h := NewListings(reader)

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

func TestListingsFacets_ReturnsDistinctMakes(t *testing.T) {
	reader := &fakeReader{makes: []string{"Chevrolet", "Mazda", "Toyota"}}
	h := NewListings(reader)

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
