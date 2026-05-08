package admin

import (
	"log/slog"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/store/pg"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminHandler provides administrative API endpoints for tenant management,
// sandbox policy configuration, and API key lifecycle operations.
type AdminHandler struct {
	store        store.Store
	pool         *pgxpool.Pool
	logger       *slog.Logger
	pricingCache *metering.PricingStore
}

// NewAdminHandler creates an AdminHandler with the given store and logger.
// If the store implements PoolProvider, the pool is extracted for transactional operations.
func NewAdminHandler(s store.Store, logger *slog.Logger, opts ...AdminOption) *AdminHandler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &AdminHandler{store: s, logger: logger}
	if pp, ok := s.(pg.PoolProvider); ok {
		h.pool = pp.Pool()
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// AdminOption configures optional dependencies for AdminHandler.
type AdminOption func(*AdminHandler)

// WithPricingCache enables cache invalidation when pricing rules are modified.
func WithPricingCache(ps *metering.PricingStore) AdminOption {
	return func(h *AdminHandler) { h.pricingCache = ps }
}

// Handler returns an http.Handler that serves all admin routes under /admin/v1/.
// All routes require the "admin" scope via RequireScope middleware.
func (h *AdminHandler) Handler() http.Handler {
	mux := http.NewServeMux()

	// Sandbox policy endpoints.
	mux.HandleFunc("POST /admin/v1/tenants/{id}/sandbox-policy", h.setSandboxPolicy)
	mux.HandleFunc("GET /admin/v1/tenants/{id}/sandbox-policy", h.getSandboxPolicy)
	mux.HandleFunc("DELETE /admin/v1/tenants/{id}/sandbox-policy", h.deleteSandboxPolicy)

	// API key management endpoints.
	mux.HandleFunc("GET /admin/v1/tenants/{id}/api-keys", h.listAPIKeys)
	mux.HandleFunc("POST /admin/v1/tenants/{id}/api-keys", h.createAPIKey)
	mux.HandleFunc("POST /admin/v1/tenants/{id}/api-keys/{kid}/rotate", h.rotateAPIKey)
	mux.HandleFunc("DELETE /admin/v1/tenants/{id}/api-keys/{kid}", h.revokeAPIKey)

	// Pricing rule management endpoints.
	mux.HandleFunc("GET /admin/v1/pricing-rules", h.listPricingRules)
	mux.HandleFunc("PUT /admin/v1/pricing-rules/{model}", h.upsertPricingRule)
	mux.HandleFunc("DELETE /admin/v1/pricing-rules/{model}", h.deletePricingRule)

	// Audit log query endpoint (cross-tenant).
	mux.HandleFunc("GET /admin/v1/audit-logs", h.listAuditLogs)

	// Wrap with RequireScope("admin") middleware.
	return middleware.RequireScope("admin")(mux)
}
