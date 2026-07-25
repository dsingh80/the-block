package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// fakeSessionStore is a minimal in-memory sessions.Store for this middleware's
// own unit tests -- no real Redis needed to test the HTTP-layer wiring.
type fakeSessionStore struct {
	byToken map[string]domain.Session
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{byToken: make(map[string]domain.Session)}
}

func (f *fakeSessionStore) Touch(_ context.Context, token string) (domain.Session, bool, error) {
	if token != "" {
		if sess, ok := f.byToken[token]; ok {
			return sess, false, nil
		}
	}
	newToken := fmt.Sprintf("fake-token-%d", len(f.byToken)+1)
	sess := domain.Session{Token: newToken, CreatedAt: time.Now(), LastSeenAt: time.Now()}
	f.byToken[newToken] = sess
	return sess, true, nil
}

func TestSession_FirstRequestIssuesACorrectlyFlaggedCookie(t *testing.T) {
	store := newFakeSessionStore()
	var sessInHandler domain.Session
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessInHandler, _ = SessionFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Session(store)(next).ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want exactly 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != SessionCookieName {
		t.Errorf("cookie name = %q, want %q", c.Name, SessionCookieName)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags = HttpOnly=%v Secure=%v SameSite=%v, want all set to HttpOnly/Secure/Lax", c.HttpOnly, c.Secure, c.SameSite)
	}
	if !c.Expires.IsZero() || c.MaxAge != 0 {
		t.Errorf("cookie has Expires=%v MaxAge=%d, want neither set (Redis's TTL is the only source of truth)", c.Expires, c.MaxAge)
	}
	if sessInHandler.Token != c.Value {
		t.Errorf("handler saw session token %q, want it to match the issued cookie %q", sessInHandler.Token, c.Value)
	}
}

// erroringSessionStore always fails Touch -- simulates Redis being unreachable.
type erroringSessionStore struct{ err error }

func (f erroringSessionStore) Touch(_ context.Context, _ string) (domain.Session, bool, error) {
	return domain.Session{}, false, f.err
}

func TestSession_StoreErrorReturns500WithoutCallingNext(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Session(erroringSessionStore{err: errors.New("redis down")})(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when store.Touch errors", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("got %d Set-Cookie headers, want 0 when session resolution failed", len(rec.Result().Cookies()))
	}
	if nextCalled {
		t.Error("next handler was called, want the chain to stop on a store error")
	}
}

func TestSession_ValidCookieGetsNoReissuedSetCookie(t *testing.T) {
	store := newFakeSessionStore()
	existing, _, _ := store.Touch(context.Background(), "")

	var sessInHandler domain.Session
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessInHandler, _ = SessionFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: existing.Token})
	rec := httptest.NewRecorder()
	Session(store)(next).ServeHTTP(rec, req)

	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("got %d Set-Cookie headers for a valid existing session, want 0", len(rec.Result().Cookies()))
	}
	if sessInHandler.Token != existing.Token {
		t.Errorf("handler saw token %q, want the existing session's %q preserved", sessInHandler.Token, existing.Token)
	}
}
