// Package httputil is the shared, dependency-light home for writing an HTTP
// JSON response -- both internal/transport/http/handlers and
// internal/transport/http/middleware need this, and middleware is imported
// by the parent transport/http package (router.go) to build the chain, so
// respond.go can't live there without an import cycle. This package only
// depends on transport/dto, so both can import it freely.
package httputil

import (
	"encoding/json"
	"net/http"

	"github.com/dsingh80/the-block/server/internal/transport/dto"
)

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError writes the standard error envelope. message should already be
// plain, currency/formatting-agnostic English -- number formatting is a
// client-side concern, same as the rest of this API's DTOs
// (guidelines/06-backend-architecture.md).
func WriteError(w http.ResponseWriter, status int, code, message string, details map[string]any, requestID string) {
	WriteJSON(w, status, dto.ErrorResponse{Error: dto.ErrorBody{
		Code: code, Message: message, Details: details, RequestID: requestID,
	}})
}
