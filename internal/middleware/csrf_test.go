package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

func TestCSRFMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		authMethod string
		header     string
		cookie     string
		hash       string
		wantStatus int
	}{
		{
			name:       "bearer post bypasses csrf",
			method:     http.MethodPost,
			authMethod: "api_key",
			wantStatus: http.StatusOK,
		},
		{
			name:       "channel get bypasses csrf",
			method:     http.MethodGet,
			authMethod: auth.ChannelAuthMethod,
			wantStatus: http.StatusOK,
		},
		{
			name:       "channel post missing csrf rejected",
			method:     http.MethodPost,
			authMethod: auth.ChannelAuthMethod,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "channel post mismatched header cookie rejected",
			method:     http.MethodPost,
			authMethod: auth.ChannelAuthMethod,
			header:     "csrf-a",
			cookie:     "csrf-b",
			hash:       auth.HashKey("csrf-a"),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "channel post hash mismatch rejected",
			method:     http.MethodPost,
			authMethod: auth.ChannelAuthMethod,
			header:     "csrf-a",
			cookie:     "csrf-a",
			hash:       auth.HashKey("csrf-b"),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "channel post valid csrf allowed",
			method:     http.MethodPost,
			authMethod: auth.ChannelAuthMethod,
			header:     "csrf-ok",
			cookie:     "csrf-ok",
			hash:       auth.HashKey("csrf-ok"),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := CSRFMiddleware(inner)
			req := httptest.NewRequest(tt.method, "/v1/test", nil)
			if tt.header != "" {
				req.Header.Set(csrfHeader, tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: auth.ChannelCSRFCookie, Value: tt.cookie})
			}
			req = req.WithContext(auth.WithContext(req.Context(), &auth.AuthContext{
				AuthMethod: tt.authMethod,
				CSRFHash:   tt.hash,
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
