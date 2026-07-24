// Package http is the HTTP transport: routing, middleware, and handlers
// (guidelines/06-backend-architecture.md). Real routes are registered
// incrementally as their handlers land in later commits.
package http

import "net/http"

// Chain applies middleware around h in the given order, outermost first --
// Chain(h, A, B) means a request passes through A, then B, then h.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
