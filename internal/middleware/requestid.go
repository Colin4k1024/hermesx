package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

type requestIDKey struct{}

var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

const maxRequestIDLen = 64

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// RequestIDMiddleware injects a unique request ID into the context and response header.
// Reuses X-Request-ID from the client if present and valid.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > maxRequestIDLen || !validRequestID.MatchString(id) {
			id = generateID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "fallback-" + hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
