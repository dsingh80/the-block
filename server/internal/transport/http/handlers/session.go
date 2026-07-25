package handlers

import (
	"net/http"
	"time"

	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
)

// SessionInfo handles GET /v1/session -- introspection only. Every request
// already gets a session resolved by middleware.Session (guidelines/06-backend-architecture.md,
// "Sessions & security"); this endpoint has no side effect of its own, it just
// lets the client ask "what does the server think my session is."
func SessionInfo(w http.ResponseWriter, r *http.Request) {
	sess, _ := middleware.SessionFromContext(r.Context())
	httputil.WriteJSON(w, http.StatusOK, dto.SessionInfo{CreatedAt: sess.CreatedAt.UTC().Format(time.RFC3339)})
}
