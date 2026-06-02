package middleware

import (
	"net/http"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

const csrfHeader = "X-Hermes-CSRF"

// CSRFMiddleware protects cookie-backed channel sessions on unsafe methods.
// Bearer-based API clients keep their existing behavior.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac, ok := auth.FromContext(r.Context())
		if !ok || ac == nil || ac.AuthMethod != auth.ChannelAuthMethod || safeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		headerToken := strings.TrimSpace(r.Header.Get(csrfHeader))
		cookie, err := r.Cookie(auth.ChannelCSRFCookie)
		if err != nil || headerToken == "" || cookie.Value == "" || headerToken != cookie.Value {
			http.Error(w, "csrf token required", http.StatusForbidden)
			return
		}
		if auth.HashKey(headerToken) != ac.CSRFHash {
			http.Error(w, "csrf token invalid", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
