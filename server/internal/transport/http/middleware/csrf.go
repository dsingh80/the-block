package middleware

import "net/http"

// CSRFHeaderName/Value: requiring this on state-changing requests is a cheap
// second layer behind SameSite=Lax -- a simple cross-site form/request can't
// attach a custom header (guidelines/06-backend-architecture.md). This is
// defense-in-depth even though there's no login yet: a malicious cross-site
// page placing a griefing bid as an unwitting visitor's anonymous session is
// a real abuse vector regardless of auth.
const (
	CSRFHeaderName  = "X-Requested-With"
	CSRFHeaderValue = "XHR"
)

// CSRF rejects a state-changing request (POST/PUT/PATCH/DELETE) missing the
// required header. Read-only requests are never blocked.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStateChanging(r.Method) && r.Header.Get(CSRFHeaderName) != CSRFHeaderValue {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"csrf_check_failed","message":"Missing or invalid ` + CSRFHeaderName + ` header."}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
