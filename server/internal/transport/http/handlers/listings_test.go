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

// fakeReader is a spy: it records the last Filter it was called with (so
// tests can assert the handler wired query params correctly) and returns a
// fixed dataset -- SQL filter semantics themselves are covered by the
// pgstore integration test, not re-tested here.
type fakeReader struct {
	all        []domain.Listing
	makes      []string
	err        error
	lastFilter listings.Filter
}

func (f *fakeReader) Get(_ context.Context, id string) (domain.Listing, error) {
	for _, l := range f.all {
		if l.ID == id {
			return l, nil
		}
	}
	return domain.Listing{}, domain.ErrNotFound
}

func (f *fakeReader) ListFiltered(_ context.Context, filter listings.Filter) ([]domain.Listing, error) {
	f.lastFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
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

func TestListingsList_WiresQueryParamsIntoFilter(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	h := NewListings(reader)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?status=active&make=Mazda&q=cx-5", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	want := listings.Filter{Status: "active", Make: "Mazda", Search: "cx-5"}
	if reader.lastFilter != want {
		t.Errorf("ListFiltered called with %+v, want %+v", reader.lastFilter, want)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListingsList_NoQueryParamsMeansNoFilter(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
	h := NewListings(reader)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if reader.lastFilter != (listings.Filter{}) {
		t.Errorf("ListFiltered called with %+v, want the zero-value Filter", reader.lastFilter)
	}
}

func TestListingsList_SerializesDataAndNeverIncludesReservePrice(t *testing.T) {
	reader := &fakeReader{all: []domain.Listing{sampleDomainListing()}}
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
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.Data) != 1 || body.Data[0].ID != "abc-123" || body.Data[0].Make != "Mazda" {
		t.Errorf("data = %+v, want exactly the one fake listing", body.Data)
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
