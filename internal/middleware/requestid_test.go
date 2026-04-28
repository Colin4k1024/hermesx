package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string // incoming X-Request-ID; empty means not set
		wantReused  bool   // whether the response should echo back the same value
	}{
		{
			name:        "generates ID when header absent",
			headerValue: "",
			wantReused:  false,
		},
		{
			name:        "reuses existing X-Request-ID",
			headerValue: "my-custom-req-id-12345",
			wantReused:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedID string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedID = RequestIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := RequestIDMiddleware(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Request-ID", tt.headerValue)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			respID := rec.Header().Get("X-Request-ID")
			if respID == "" {
				t.Fatal("expected X-Request-ID in response header, got empty")
			}

			if capturedID == "" {
				t.Fatal("expected request ID in context, got empty")
			}

			if capturedID != respID {
				t.Errorf("context ID %q != response header ID %q", capturedID, respID)
			}

			if tt.wantReused {
				if respID != tt.headerValue {
					t.Errorf("response ID = %q, want reused %q", respID, tt.headerValue)
				}
			} else {
				if !hexPattern.MatchString(respID) {
					t.Errorf("generated ID %q does not match 32-char hex pattern", respID)
				}
			}
		})
	}
}

func TestRequestIDFromContext_EmptyContext(t *testing.T) {
	ctx := context.Background()
	if got := RequestIDFromContext(ctx); got != "" {
		t.Errorf("RequestIDFromContext on empty context = %q, want empty", got)
	}
}
