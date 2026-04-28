package middleware

import "net/http"

// Middleware wraps an http.Handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// StackConfig configures which middleware layers are active.
type StackConfig struct {
	Auth      Middleware
	Tenant    Middleware
	RBAC      Middleware
	RateLimit Middleware
	RequestID Middleware
	Metrics   Middleware
}

// MiddlewareStack enforces a fixed middleware ordering across all HTTP servers.
// Execution order (outermost first): Metrics → RequestID → Auth → Tenant → RBAC → RateLimit → Handler.
type MiddlewareStack struct {
	cfg StackConfig
}

// NewStack creates a MiddlewareStack from the given config.
// Nil middleware slots are treated as passthrough (no-op).
func NewStack(cfg StackConfig) *MiddlewareStack {
	return &MiddlewareStack{cfg: cfg}
}

// Wrap applies the full middleware chain to the given handler.
func (s *MiddlewareStack) Wrap(handler http.Handler) http.Handler {
	h := handler
	h = apply(s.cfg.RateLimit, h)
	h = apply(s.cfg.RBAC, h)
	h = apply(s.cfg.Tenant, h)
	h = apply(s.cfg.Auth, h)
	h = apply(s.cfg.RequestID, h)
	h = apply(s.cfg.Metrics, h)
	return h
}

func apply(mw Middleware, h http.Handler) http.Handler {
	if mw == nil {
		return h
	}
	return mw(h)
}
