package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/usecase/bidding"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
	"github.com/dsingh80/the-block/server/internal/usecase/realtime"
)

// The stubs below satisfy NewRouter's ports with the bare minimum to route a
// request -- their own behavior is already covered where each port is tested
// (pgstore, redisstore, the handlers package). This file is only about
// router.go's own wiring: which middleware chain each route actually runs
// through.

type stubListingReader struct{}

func (stubListingReader) Get(context.Context, string) (domain.Listing, error) {
	return domain.Listing{}, domain.ErrNotFound
}
func (stubListingReader) DistinctMakes(context.Context) ([]string, error) { return nil, nil }
func (stubListingReader) ListPage(context.Context, listings.PageRequest) (listings.Page, error) {
	return listings.Page{}, nil
}

type stubBidReader struct{}

func (stubBidReader) ListForListing(context.Context, string, int) ([]domain.Bid, error) {
	return nil, nil
}

type stubViewerLookup struct{}

func (stubViewerLookup) BidListingIDs(context.Context, string) (map[string]struct{}, error) {
	return nil, nil
}

func (stubViewerLookup) HighBidderSessions(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

type stubBidStore struct{}

func (stubBidStore) PlaceBid(context.Context, string, string, int64) (bidding.Result, error) {
	return bidding.Result{}, nil
}
func (stubBidStore) BuyNow(context.Context, string, string) (bidding.Result, error) {
	return bidding.Result{}, nil
}

type stubRateLimiter struct{}

func (stubRateLimiter) Allow(context.Context, string, string) (bool, error) { return true, nil }

// stubSessionStore always mints a "new" session -- isNew=true is what makes
// middleware.Session issue a Set-Cookie, which is exactly the signal
// TestNewRouter_HealthcheckBypassesSessionMiddleware checks is absent.
type stubSessionStore struct{}

func (stubSessionStore) Touch(context.Context, string) (domain.Session, bool, error) {
	return domain.Session{Token: "stub-token", CreatedAt: time.Now()}, true, nil
}

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

type stubBroadcaster struct{}

func (stubBroadcaster) Subscribe(realtime.Subscriber, ...string)   {}
func (stubBroadcaster) Unsubscribe(realtime.Subscriber, ...string) {}
func (stubBroadcaster) Publish(context.Context, realtime.Event)    {}

func newTestRouter(redisErr, postgresErr error) http.Handler {
	return NewRouter(
		stubListingReader{}, stubBidReader{}, stubViewerLookup{},
		stubBidStore{}, stubRateLimiter{}, stubSessionStore{},
		stubPinger{err: redisErr}, stubPinger{err: postgresErr},
		stubBroadcaster{}, []string{"https://localhost"},
	)
}

func TestNewRouter_HealthcheckBypassesSessionMiddleware(t *testing.T) {
	router := newTestRouter(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("healthcheck response set cookies %v, want none -- a liveness probe must not spin up a session", cookies)
	}
}

func TestNewRouter_HealthcheckReports503WhenADependencyIsUnreachable(t *testing.T) {
	router := newTestRouter(errors.New("redis down"), nil)

	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestNewRouter_OrdinaryRoutesStillGetASession(t *testing.T) {
	router := newTestRouter(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != middleware.SessionCookieName {
		t.Errorf("cookies = %v, want exactly one %q cookie -- ordinary routes must still go through Session middleware", cookies, middleware.SessionCookieName)
	}
}

func TestNewRouter_ListingsRouteIsReachable(t *testing.T) {
	router := newTestRouter(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}
