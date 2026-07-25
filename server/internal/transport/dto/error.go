// Package dto holds wire structs and domain<->DTO mapping, transport-agnostic
// by construction (guidelines/06-backend-architecture.md) -- nothing here
// imports net/http or any framework, so the same structs could serve a future
// transport unchanged.
package dto

// ErrorResponse is the wire shape for every 4xx/5xx response
// (guidelines/06-backend-architecture.md).
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}
