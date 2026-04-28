package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
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
}

// APIServer is the multi-tenant SaaS API HTTP server.
type APIServer struct {
	cfg    APIServerConfig
	server *http.Server
}

// NewAPIServer creates and configures the API server with all routes and middleware.
func NewAPIServer(cfg APIServerConfig) *APIServer {
	stack := middleware.NewStack(middleware.StackConfig{
		Metrics:   middleware.MetricsMiddleware,
		RequestID: middleware.RequestIDMiddleware,
		Auth:      middleware.AuthMiddleware(cfg.AuthChain, false),
		Tenant:    middleware.TenantMiddleware,
		RBAC:      middleware.RBACMiddleware(cfg.RBAC),
		RateLimit: middleware.RateLimitMiddleware(cfg.RateLimit),
	})

	auditMW := middleware.AuditMiddleware(cfg.Store.AuditLogs())

	mux := http.NewServeMux()

	// Public routes — no middleware stack.
	health := NewHealthHandler(cfg.DB)
	mux.HandleFunc("GET /health/live", health.LiveHandler())
	mux.HandleFunc("GET /health/ready", health.ReadyHandler())
	mux.Handle("GET /metrics", promhttp.Handler())

	// Authenticated routes — through middleware stack + audit.
	api := http.NewServeMux()
	api.Handle("/v1/tenants", NewTenantHandler(cfg.Store.Tenants()))
	api.Handle("/v1/tenants/", NewTenantHandler(cfg.Store.Tenants()))
	api.Handle("/v1/api-keys", NewAPIKeyHandler(cfg.Store.APIKeys()))
	api.Handle("/v1/api-keys/", NewAPIKeyHandler(cfg.Store.APIKeys()))
	api.Handle("/v1/audit-logs", NewAuditHandler(cfg.Store.AuditLogs()))
	api.Handle("/v1/usage", NewUsageHandler(cfg.Store.Sessions(), cfg.Store.Messages()))
	api.HandleFunc("GET /v1/openapi", OpenAPISpec())

	gdpr := NewGDPRHandler(cfg.Store.Sessions(), cfg.Store.Messages())
	api.HandleFunc("GET /v1/gdpr/export", gdpr.ExportHandler())
	api.HandleFunc("DELETE /v1/gdpr/data", gdpr.DeleteHandler())

	mux.Handle("/v1/", auditMW(stack.Wrap(api)))

	s := &APIServer{
		cfg: cfg,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
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

// Shutdown gracefully stops the server.
func (s *APIServer) Shutdown(ctx context.Context) error {
	slog.Info("SaaS API server shutting down")
	return s.server.Shutdown(ctx)
}
