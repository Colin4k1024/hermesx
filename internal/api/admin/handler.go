package admin

import (
	"log/slog"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/mcpcatalog"
	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/store/pg"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminHandler provides administrative API endpoints for tenant management,
// sandbox policy configuration, and API key lifecycle operations.
type AdminHandler struct {
	store          store.Store
	pool           *pgxpool.Pool
	logger         *slog.Logger
	pricingCache   *metering.PricingStore
	usageStore     metering.UsageStore
	evolutionStore *evolution.GeneStore
	mcpCatalog     mcpcatalog.Store
	egressHandler  *egress.AdminHandler
	policyStore    safety.PolicyStore
	canaryDetector *safety.CanaryDetector
	leakScanner    *secrets.LeakScanner
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

// WithUsageStore enables the admin usage aggregation endpoint.
func WithUsageStore(us metering.UsageStore) AdminOption {
	return func(h *AdminHandler) { h.usageStore = us }
}

// WithEvolutionStore enables evolution sharing governance endpoints.
func WithEvolutionStore(gs *evolution.GeneStore) AdminOption {
	return func(h *AdminHandler) { h.evolutionStore = gs }
}

// WithMCPCatalog enables MCP catalog governance endpoints.
func WithMCPCatalog(store mcpcatalog.Store) AdminOption {
	return func(h *AdminHandler) { h.mcpCatalog = store }
}

// WithEgressHandler registers the egress admin handler so its routes are
// served under /admin/v1/egress/... with the admin domain-scope guard.
func WithEgressHandler(eh *egress.AdminHandler) AdminOption {
	return func(h *AdminHandler) { h.egressHandler = eh }
}

// WithPolicyStore wires the safety policy store into the admin handler (M-8).
func WithPolicyStore(ps safety.PolicyStore) AdminOption {
	return func(h *AdminHandler) { h.policyStore = ps }
}

// WithCanaryDetector wires the canary detector into the admin handler (M-8).
func WithCanaryDetector(cd *safety.CanaryDetector) AdminOption {
	return func(h *AdminHandler) { h.canaryDetector = cd }
}

// WithLeakScanner wires the secret leak scanner into the admin handler (M-8).
func WithLeakScanner(ls *secrets.LeakScanner) AdminOption {
	return func(h *AdminHandler) { h.leakScanner = ls }
}

// Handler returns an http.Handler that serves all admin routes under /admin/v1/.
// Routes are guarded by domain-specific scopes, with the legacy "admin" scope
// retained as an explicit break-glass compatibility grant.
func (h *AdminHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, scopes []string, fn http.HandlerFunc) {
		mux.Handle(pattern, middleware.RequireAnyScope(scopes...)(fn))
	}

	// Sandbox policy endpoints.
	handle("POST /admin/v1/tenants/{id}/sandbox-policy", []string{"ops:write", "tenant:write"}, h.setSandboxPolicy)
	handle("GET /admin/v1/tenants/{id}/sandbox-policy", []string{"ops:read", "tenant:read"}, h.getSandboxPolicy)
	handle("DELETE /admin/v1/tenants/{id}/sandbox-policy", []string{"ops:write", "tenant:write"}, h.deleteSandboxPolicy)

	// API key management endpoints.
	handle("GET /admin/v1/tenants/{id}/api-keys", []string{"key:read", "tenant:read"}, h.listAPIKeys)
	handle("POST /admin/v1/tenants/{id}/api-keys", []string{"key:write"}, h.createAPIKey)
	handle("POST /admin/v1/tenants/{id}/api-keys/{kid}/rotate", []string{"key:write"}, h.rotateAPIKey)
	handle("DELETE /admin/v1/tenants/{id}/api-keys/{kid}", []string{"key:write"}, h.revokeAPIKey)

	// Pricing rule management endpoints.
	handle("GET /admin/v1/pricing-rules", []string{"billing:read"}, h.listPricingRules)
	handle("PUT /admin/v1/pricing-rules/{model}", []string{"billing:write"}, h.upsertPricingRule)
	handle("DELETE /admin/v1/pricing-rules/{model}", []string{"billing:write"}, h.deletePricingRule)

	// Audit log query endpoint (cross-tenant).
	handle("GET /admin/v1/audit-logs", []string{"audit:read"}, h.listAuditLogs)

	// Trusted channel login configuration and binding governance.
	handle("GET /admin/v1/channel-apps", []string{"channel:read"}, h.listChannelApps)
	handle("POST /admin/v1/channel-apps", []string{"channel:write"}, h.createChannelApp)
	handle("PATCH /admin/v1/channel-apps/{id}", []string{"channel:write"}, h.updateChannelApp)
	handle("DELETE /admin/v1/channel-apps/{id}", []string{"channel:write"}, h.deleteChannelApp)
	handle("GET /admin/v1/channel-bindings", []string{"channel:read"}, h.listChannelBindings)
	handle("DELETE /admin/v1/channel-bindings/{id}", []string{"channel:write"}, h.revokeChannelBinding)

	// Tenant usage aggregation (cross-tenant aggregate-only view).
	handle("GET /admin/v1/usage/tenants", []string{"billing:read", "usage:read:all"}, h.listTenantUsage)

	// Per-tenant usage aggregation via metering store.
	handle("GET /admin/v1/usage", []string{"billing:read", "usage:read"}, h.adminUsageAggregation)

	// Evolution shared learning governance endpoints.
	handle("GET /admin/v1/evolution/sharing-policy", []string{"sharing:read", "security:read"}, h.getEvolutionSharingPolicy)
	handle("GET /admin/v1/evolution/sharing-policy/history", []string{"sharing:read", "security:read"}, h.listEvolutionSharingPolicyHistory)
	handle("PUT /admin/v1/evolution/sharing-policy", []string{"sharing:write", "security:write"}, h.updateEvolutionSharingPolicy)
	handle("POST /admin/v1/evolution/sharing-policy/rollback", []string{"sharing:write", "security:write"}, h.rollbackEvolutionSharingPolicy)
	handle("GET /admin/v1/evolution/tenants/{id}/sharing-policy", []string{"sharing:read", "tenant:read"}, h.getEvolutionTenantSharingPolicy)
	handle("GET /admin/v1/evolution/tenants/{id}/sharing-policy/history", []string{"sharing:read", "tenant:read"}, h.listEvolutionTenantSharingPolicyHistory)
	handle("PUT /admin/v1/evolution/tenants/{id}/sharing-policy", []string{"sharing:write", "tenant:write"}, h.updateEvolutionTenantSharingPolicy)
	handle("POST /admin/v1/evolution/tenants/{id}/sharing-policy/rollback", []string{"sharing:write", "tenant:write"}, h.rollbackEvolutionTenantSharingPolicy)
	handle("POST /admin/v1/evolution/shared-knowledge/revoke", []string{"sharing:write", "security:write"}, h.revokeEvolutionSharedKnowledge)

	// Secret pattern management endpoints.
	handle("GET /admin/v1/secrets/patterns", []string{"security:read"}, h.listSecretPatterns)
	handle("POST /admin/v1/secrets/patterns", []string{"security:write"}, h.createSecretPattern)

	// Canary token management endpoints.
	handle("GET /admin/v1/secrets/canary-tokens", []string{"security:read"}, h.listCanaryTokens)
	handle("DELETE /admin/v1/secrets/canary-tokens/{id}", []string{"security:write"}, h.deleteCanaryToken)

	// Safety policy management endpoints.
	handle("GET /admin/v1/safety/rules", []string{"security:read"}, h.listSafetyRules)
	handle("PUT /admin/v1/safety/rules/{id}", []string{"security:write"}, h.updateSafetyRule)
	handle("POST /admin/v1/safety/scan", []string{"security:write"}, h.safetyManualScan)

	// Governed MCP catalog endpoints.
	handle("GET /admin/v1/mcp-catalog", []string{"security:read", "ops:read"}, h.listMCPCatalogItems)
	handle("GET /admin/v1/mcp-catalog/{id}", []string{"security:read", "ops:read"}, h.getMCPCatalogItem)
	handle("PUT /admin/v1/mcp-catalog/{id}", []string{"security:write", "ops:write"}, h.upsertMCPCatalogItem)
	handle("GET /admin/v1/mcp-catalog/tenants/{id}", []string{"security:read", "tenant:read"}, h.listMCPTenantPolicies)
	handle("PUT /admin/v1/mcp-catalog/tenants/{id}/items/{itemID}", []string{"security:write", "tenant:write"}, h.setMCPTenantPolicy)

	// Egress allowlist management endpoints (delegated to egress.AdminHandler).
	if h.egressHandler != nil {
		egressMux := http.NewServeMux()
		h.egressHandler.RegisterV1Routes(egressMux)
		mux.Handle("/admin/v1/egress/", middleware.RequireAnyScope("security:write", "ops:write")(egressMux))
	}

	return mux
}
