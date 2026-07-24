package middleware

import (
	"context"
	"net/http"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/platform/logging"
	"github.com/dsingh80/the-block/server/internal/usecase/sessions"
)

// SessionCookieName is the opaque, unauthenticated identity cookie
// (guidelines/06-backend-architecture.md, "Sessions & security").
const SessionCookieName = "block_sid"

type sessionContextKey int

const sessionKey sessionContextKey = iota

// Session ensures every request has a session (creating one via store.Touch if
// there's no cookie, or the cookie is unknown/expired), makes it available via
// SessionFromContext, and issues a Set-Cookie only when a new session was
// actually created. Deliberately no Max-Age/Expires on the cookie -- Redis's
// own TTL is the single source of truth for validity, so giving the cookie its
// own expiry too would create two clocks that could disagree
// (guidelines/06-backend-architecture.md).
func Session(store sessions.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			if c, err := r.Cookie(SessionCookieName); err == nil {
				token = c.Value
			}

			sess, isNew, err := store.Touch(r.Context(), token)
			if err != nil {
				logging.FromContext(r.Context()).Error("session touch failed", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if isNew {
				http.SetCookie(w, &http.Cookie{
					Name:     SessionCookieName,
					Value:    sess.Token,
					Path:     "/",
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				})
			}

			ctx := context.WithValue(r.Context(), sessionKey, sess)
			// session_id is an opaque, non-PII lookup key (guidelines/06-backend-architecture.md,
			// "SOC2 principles mapping") -- safe to carry on every log line from
			// here down, same as request_id.
			ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("session_id", sess.Token))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionFromContext(ctx context.Context) (domain.Session, bool) {
	sess, ok := ctx.Value(sessionKey).(domain.Session)
	return sess, ok
}
