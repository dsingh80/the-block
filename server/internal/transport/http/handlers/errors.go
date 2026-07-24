package handlers

import (
	"errors"
	"net/http"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/transport/http/middleware"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
)

// domainErrorResponses maps a domain.DomainError's Code to the HTTP status and
// plain, currency/formatting-agnostic message to send (guidelines/06-backend-architecture.md).
// Grows as later handlers introduce codes that need it (not_found, bid_too_low,
// etc.) rather than being pre-populated for codes nothing produces yet.
var domainErrorResponses = map[string]struct {
	status  int
	message string
}{
	"not_found":            {http.StatusNotFound, "That listing doesn't exist."},
	"invalid_cursor":       {http.StatusBadRequest, "This page link is invalid."},
	"cursor_sort_mismatch": {http.StatusBadRequest, "This page link doesn't match the current filter or sort -- start over from the first page."},
}

// writeError writes err as the standard envelope: a recognized domain.DomainError
// maps to its specific status/message/details, anything else is a generic 500.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := middleware.RequestIDFromContext(r.Context())

	var de *domain.DomainError
	if errors.As(err, &de) {
		if resp, ok := domainErrorResponses[de.Code]; ok {
			httputil.WriteError(w, resp.status, de.Code, resp.message, de.Details, requestID)
			return
		}
	}
	httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.", nil, requestID)
}
