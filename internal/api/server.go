package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/api/admin"
	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/store/pg"
	"github.com/Colin4k1024/hermesx/internal/tools"
	workflowrt "github.com/Colin4k1024/hermesx/internal/workflow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// APIServerConfig holds all dependencies for the SaaS API server.
type APIServerConfig struct {
	Port      int
	Store     store.Store
	DB        DBPinger // optional; nil disables readiness DB check
	AuthChain *auth.ExtractorChain
	RBAC      middleware.RBACConfig
	RateLimit middleware.RateLimitConfig
	// BootstrapRateLimitRPM limits unauthenticated POST /admin/v1/bootstrap attempts by source IP.
	// A value <= 0 uses the secure default of 5 requests/minute.
	BootstrapRateLimitRPM int
	AllowedOrigins        string                // comma-separated list of allowed origins, or "*" for all
	StaticDir             string                // directory to serve static files from (optional)
	SkillsClient          objstore.ObjectStore  // optional; nil disables per-tenant skill loading
	Provisioner           *skills.Provisioner   // optional; nil disables per-user skill provisioning
	UsageStore            metering.UsageStore   // optional; enables usage_records-backed usage APIs
	EvolutionStore        *evolution.GeneStore  // optional; enables evolution sharing governance APIs
	TenantOpts            []TenantHandlerOption // optional; wired into TenantHandler on creation
}

// APIServer is the multi-tenant SaaS API HTTP server.
type APIServer struct {
	cfg              APIServerConfig
	server           *http.Server
	backgroundCancel context.CancelFunc
	AgentChat        *chatHandler
}

// spaFallback wraps the API mux: serves index.html for root "/" and admin.html
// for the Admin Console entry, delegating all other paths to the inner mux.
func spaFallback(mux http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "/index.html" {
			http.ServeFile(w, r, staticDir+"/index.html")
			return
		}
		if path == "/admin.html" {
			http.ServeFile(w, r, staticDir+"/admin.html")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// corsMiddleware returns an HTTP handler that adds CORS headers.
func corsMiddleware(next http.Handler, origins string) http.Handler {
	allowAll := origins == "*"
	allowed := make(map[string]bool)
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Always add Vary: Origin so caches don't serve a cached CORS response to a different origin.
		w.Header().Add("Vary", "Origin")
		if origin != "" && (allowAll || allowed[origin]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Hermes-Session-Id, X-Hermes-User-Id")
			w.Header().Set("Access-Control-Expose-Headers", "X-Hermes-Session-Id, X-Request-ID")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func egressStores(s store.Store) (egress.RuleStore, egress.RuleAdminStore) {
	if pp, ok := s.(pg.PoolProvider); ok && pp.Pool() != nil {
		egressStore := egress.NewPGXStore(pp.Pool())
		return egressStore, egressStore
	}
	return egress.EmptyRuleStore{}, nil
}

// NewAPIServer creates and configures the API server with all routes and middleware.
func NewAPIServer(cfg APIServerConfig) *APIServer {
	egressStore, egressAdminStore := egressStores(cfg.Store)
	egressPolicy, egressDefault, err := egress.NewAllowlistPolicyFromEnv(egressStore, nil)
	if err != nil {
		slog.Error("invalid egress default policy", "error", err)
		egressPolicy = egress.NewAllowlistPolicy(egressStore, nil, egress.DefaultDenyAll)
		egressDefault = egress.DefaultDenyAll
	}
	cachedEgressPolicy := egress.NewCachedPolicy(egressPolicy, time.Minute)
	egressTransport := egress.NewSecureTransport(cachedEgressPolicy)
	slog.Info("egress policy configured", "default", egressDefault, "tenant_allowlist", egressAdminStore != nil)

	stack := middleware.NewStack(middleware.StackConfig{
		Tracing:   middleware.TracingMiddleware,
		Metrics:   middleware.MetricsMiddleware,
		RequestID: middleware.RequestIDMiddleware,
		Auth:      middleware.AuthMiddleware(cfg.AuthChain, false, cfg.Store.AuditLogs()),
		Tenant:    middleware.TenantMiddleware,
		Audit:     middleware.AuditMiddleware(cfg.Store.AuditLogs()),
		RBAC:      middleware.RBACMiddleware(cfg.RBAC),
		RateLimit: middleware.RateLimitMiddleware(cfg.RateLimit),
	})

	leakScanner := secrets.NewLeakScanner()
	canaryDetector := safety.NewCanaryDetector()
	policyStore := safety.PolicyStore(safety.NewInMemoryPolicyStore())
	if pp, ok := cfg.Store.(pg.PoolProvider); ok && pp.Pool() != nil {
		policyStore = safety.NewPostgresPolicyStore(pp.Pool())
	}
	safetyInterceptor := safety.NewInterceptorChainWithCanary(policyStore, canaryDetector)

	mux := http.NewServeMux()

	// Public routes — no middleware stack.
	health := NewHealthHandler(cfg.DB)
	mux.HandleFunc("GET /health/live", health.LiveHandler())
	mux.HandleFunc("GET /health/ready", health.ReadyHandler())
	mux.Handle("GET /metrics", promhttp.Handler())

	// Authenticated routes — through middleware stack + audit.
	api := http.NewServeMux()
	tenantHandler := NewTenantHandler(cfg.Store.Tenants(), cfg.TenantOpts...)
	api.Handle("/v1/tenants", tenantHandler)
	api.Handle("/v1/tenants/", tenantHandler)
	api.Handle("/v1/api-keys", NewAPIKeyHandler(cfg.Store.APIKeys()))
	api.Handle("/v1/api-keys/", NewAPIKeyHandler(cfg.Store.APIKeys()))
	api.Handle("/v1/audit-logs", NewAuditHandler(cfg.Store.AuditLogs()))
	api.Handle("/v1/execution-receipts", NewExecutionReceiptHandler(cfg.Store.ExecutionReceipts()))
	api.Handle("/v1/execution-receipts/", NewExecutionReceiptHandler(cfg.Store.ExecutionReceipts()))
	if cfg.UsageStore != nil {
		usageH := NewUsageV2Handler(cfg.UsageStore)
		api.Handle("/v1/usage", usageH)
		api.Handle("/v1/usage/", usageH)
	} else {
		api.Handle("/v1/usage", NewUsageHandler(cfg.Store.Sessions(), cfg.Store.Messages()))
	}
	api.HandleFunc("GET /v1/openapi", OpenAPISpec())
	workflowHTTPClient := &http.Client{Transport: egressTransport, Timeout: 30 * time.Second}
	receiptRecorder := tools.NewReceiptRecorder(cfg.Store.ExecutionReceipts())
	workflowEngine := workflowrt.NewEngine(cfg.Store.Workflows(), workflowHTTPClient, workflowrt.NewDefaultAgentExecutorWithGovernanceAndSafety(egressTransport, receiptRecorder, cfg.Store.Tenants(), safetyInterceptor, leakScanner))
	workflowH := NewWorkflowHandlerWithEngine(cfg.Store.Workflows(), workflowEngine)
	api.HandleFunc("/v1/workflow-definitions", workflowH.ServeDefinitionsHTTP)
	api.HandleFunc("/v1/workflow-definitions/", workflowH.ServeDefinitionsHTTP)
	api.HandleFunc("/v1/workflow-runs", workflowH.ServeRunsHTTP)
	api.HandleFunc("/v1/workflow-runs/", workflowH.ServeRunsHTTP)
	api.HandleFunc("/v1/workflow-tasks", workflowH.ServeTasksHTTP)
	api.HandleFunc("/v1/workflow-tasks/", workflowH.ServeTasksHTTP)

	me := NewMeHandler(cfg.Store)
	api.Handle("/v1/me", me)

	gdpr := NewGDPRHandler(cfg.Store, cfg.SkillsClient)
	api.HandleFunc("GET /v1/gdpr/export", gdpr.ExportHandler())
	api.HandleFunc("DELETE /v1/gdpr/data", gdpr.DeleteHandler())
	api.HandleFunc("POST /v1/gdpr/cleanup-minio", gdpr.CleanupMinIOHandler())

	// Chat endpoint — full AIAgent with tool loop, soul, skills, memory.
	chatH := NewChatHandler(cfg.Store, cfg.SkillsClient, cfg.Provisioner)
	chatH.SetEgressTransport(egressTransport)
	chatH.SetSafetyInterceptor(safetyInterceptor)
	chatH.SetLeakScanner(leakScanner)
	chatH.SetUsageStore(cfg.UsageStore)
	api.HandleFunc("POST /v1/chat/completions", chatH.ServeAgentHTTP)
	api.HandleFunc("POST /v1/agent/chat", chatH.ServeAgentHTTP)

	// Memory management API (per-user long-term memory).
	api.HandleFunc("GET /v1/memories", chatH.handleListMemories)
	api.HandleFunc("DELETE /v1/memories/", chatH.handleDeleteMemory)

	// Session history API (per-user session and message history).
	api.HandleFunc("GET /v1/sessions", chatH.handleListUserSessions)
	api.HandleFunc("GET /v1/sessions/", chatH.handleGetSessionMessages)

	// Per-tenant skills management API.
	if cfg.SkillsClient != nil {
		skillHandler := NewSkillHandler(cfg.SkillsClient)
		api.Handle("/v1/skills", skillHandler)
		api.Handle("/v1/skills/", skillHandler)
	}

	mux.Handle("/v1/", stack.Wrap(api))

	// Bootstrap endpoints are public (status) or use ACP token (create) — no admin scope required.
	bootstrapH := admin.NewBootstrapHandler(cfg.Store, nil)
	mux.HandleFunc("GET /admin/v1/bootstrap/status", bootstrapH.Status)
	bootstrapRPM := cfg.BootstrapRateLimitRPM
	if bootstrapRPM <= 0 {
		bootstrapRPM = 5
	}
	bootstrapCreate := middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		Limiter:    cfg.RateLimit.Limiter,
		DefaultRPM: bootstrapRPM,
	})(http.HandlerFunc(bootstrapH.Create))
	mux.Handle("POST /admin/v1/bootstrap", bootstrapCreate)

	// Admin API — requires "admin" scope; uses its own sub-router with RequireScope.
	adminOpts := []admin.AdminOption{}
	adminOpts = append(adminOpts,
		admin.WithLeakScanner(leakScanner),
		admin.WithCanaryDetector(canaryDetector),
		admin.WithPolicyStore(policyStore),
	)
	if cfg.UsageStore != nil {
		adminOpts = append(adminOpts, admin.WithUsageStore(cfg.UsageStore))
	}
	if cfg.EvolutionStore != nil {
		adminOpts = append(adminOpts, admin.WithEvolutionStore(cfg.EvolutionStore))
	}
	if egressAdminStore != nil {
		adminOpts = append(adminOpts, admin.WithEgressHandler(egress.NewAdminHandler(egressAdminStore, cachedEgressPolicy)))
	}
	adminH := admin.NewAdminHandler(cfg.Store, nil, adminOpts...)
	mux.Handle("/admin/", stack.Wrap(adminH.Handler()))

	// Static file serving (optional).
	var spaHandler http.Handler
	if cfg.StaticDir != "" {
		if _, err := os.Stat(cfg.StaticDir); err == nil {
			spaHandler = http.FileServer(http.Dir(cfg.StaticDir))
			// Strip /static/ prefix.
			mux.Handle("/static/", http.StripPrefix("/static/", spaHandler))
			slog.Info("Serving static files", "dir", cfg.StaticDir)
		} else {
			slog.Warn("Static directory not found, skipping static file serving", "dir", cfg.StaticDir)
		}
	}

	var handler http.Handler = mux

	// Wrap mux with SPA fallback: serve index.html for root, else pass through to mux.
	// Done outside mux to avoid ServeMux path conflict between "/" and "/v1/".
	if spaHandler != nil {
		handler = spaFallback(handler, cfg.StaticDir)
	}

	// Apply CORS if configured.
	if cfg.AllowedOrigins != "" {
		handler = corsMiddleware(handler, cfg.AllowedOrigins)
		slog.Info("CORS enabled", "origins", cfg.AllowedOrigins)
	}

	backgroundCtx, backgroundCancel := context.WithCancel(context.Background())
	if cfg.EvolutionStore != nil {
		cfg.EvolutionStore.StartSharingPolicyWatcher(backgroundCtx, 30*time.Second)
	}

	s := &APIServer{
		cfg:              cfg,
		AgentChat:        chatH,
		backgroundCancel: backgroundCancel,
		server: &http.Server{
			Addr:         fmt.Sprintf("0.0.0.0:%d", cfg.Port),
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 150 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
	return s
}

// Start begins serving. Blocks until the server is shut down.
func (s *APIServer) Start() error {
	slog.Info("SaaS API server starting", "port", s.cfg.Port)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

// Handler returns the root HTTP handler for use with httptest or custom listeners.
func (s *APIServer) Handler() http.Handler {
	return s.server.Handler
}

// Shutdown gracefully stops the server.
func (s *APIServer) Shutdown(ctx context.Context) error {
	slog.Info("SaaS API server shutting down")
	if s.backgroundCancel != nil {
		s.backgroundCancel()
	}
	return s.server.Shutdown(ctx)
}
