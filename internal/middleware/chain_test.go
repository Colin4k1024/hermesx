package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func tracingMiddleware(name string, trace *[]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*trace = append(*trace, name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestMiddlewareStack(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(trace *[]string) StackConfig
		wantOrder []string
	}{
		{
			name: "full stack ordering",
			cfg: func(trace *[]string) StackConfig {
				return StackConfig{
					Metrics:   tracingMiddleware("Metrics", trace),
					RequestID: tracingMiddleware("RequestID", trace),
					Auth:      tracingMiddleware("Auth", trace),
					Tenant:    tracingMiddleware("Tenant", trace),
					RBAC:      tracingMiddleware("RBAC", trace),
					RateLimit: tracingMiddleware("RateLimit", trace),
				}
			},
			wantOrder: []string{"Metrics", "RequestID", "Auth", "Tenant", "RBAC", "RateLimit", "Handler"},
		},
		{
			name: "nil middleware slots are skipped",
			cfg: func(trace *[]string) StackConfig {
				return StackConfig{
					Auth:    tracingMiddleware("Auth", trace),
					Metrics: tracingMiddleware("Metrics", trace),
					// Tenant, RBAC, RateLimit, RequestID are nil
				}
			},
			wantOrder: []string{"Metrics", "Auth", "Handler"},
		},
		{
			name: "empty config passes through to handler",
			cfg: func(_ *[]string) StackConfig {
				return StackConfig{}
			},
			wantOrder: []string{"Handler"},
		},
		{
			name: "single middleware works",
			cfg: func(trace *[]string) StackConfig {
				return StackConfig{
					RateLimit: tracingMiddleware("RateLimit", trace),
				}
			},
			wantOrder: []string{"RateLimit", "Handler"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var trace []string
			cfg := tt.cfg(&trace)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace = append(trace, "Handler")
				w.WriteHeader(http.StatusOK)
			})

			stack := NewStack(cfg)
			wrapped := stack.Wrap(handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			if len(trace) != len(tt.wantOrder) {
				t.Fatalf("trace length = %d, want %d; got %v", len(trace), len(tt.wantOrder), trace)
			}
			for i, want := range tt.wantOrder {
				if trace[i] != want {
					t.Errorf("trace[%d] = %q, want %q; full trace: %v", i, trace[i], want, trace)
				}
			}
		})
	}
}
