package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/acp"
	"github.com/hermes-agent/hermes-agent-go/internal/api"
	"github.com/hermes-agent/hermes-agent-go/internal/auth"
	"github.com/hermes-agent/hermes-agent-go/internal/gateway"
	"github.com/hermes-agent/hermes-agent-go/internal/gateway/platforms"
	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/observability"
	"github.com/hermes-agent/hermes-agent-go/internal/store/rediscache"
	"github.com/hermes-agent/hermes-agent-go/internal/objstore"
	"github.com/hermes-agent/hermes-agent-go/internal/skills"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

var saasAPICmd = &cobra.Command{
	Use:   "saas-api",
	Short: "Start the SaaS multi-tenant API server",
	Long: `Starts the Hermes SaaS API server with multi-tenant auth, RBAC, rate limiting, and static file serving.

Required environment variables:
  DATABASE_URL      PostgreSQL connection URL
  HERMES_ACP_TOKEN  Static bearer token for ACP endpoints

Optional environment variables:
  SAAS_API_PORT          API server port (default: 8080)
  SAAS_ALLOWED_ORIGINS    Comma-separated CORS origins, or "*" for all
  SAAS_STATIC_DIR         Directory for static files (e.g. ./internal/dashboard/static)
  HERMES_API_PORT         OpenAI-compatible adapter port (default: 8081)
  HERMES_API_KEY          Bearer token for OpenAI-compatible adapter
  HERMES_ACP_PORT         ACP server port

Examples:
  hermes saas-api                                           # Basic
  SAAS_API_PORT=8080 SAAS_STATIC_DIR=./internal/dashboard/static hermes saas-api
`,
	RunE: runSaaSAPI,
}

func init() {
	rootCmd.AddCommand(saasAPICmd)
}

func runSaaSAPI(cmd *cobra.Command, args []string) error {
	setupLogging()

	// ── 0. OTel tracing (no-op if OTEL_EXPORTER_OTLP_ENDPOINT unset) ──
	tracerShutdown, err := observability.InitTracer(context.Background(), "hermes-agent", version)
	if err != nil {
		slog.Warn("OTel tracer init failed, continuing without tracing", "error", err)
		tracerShutdown = func(context.Context) error { return nil }
	}
	defer func() { _ = tracerShutdown(context.Background()) }()

	// ── 1. Parse environment ──────────────────────────────────
	port := 8080
	if v := os.Getenv("SAAS_API_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	allowedOrigins := os.Getenv("SAAS_ALLOWED_ORIGINS")
	staticDir := os.Getenv("SAAS_STATIC_DIR")
	acpToken := os.Getenv("HERMES_ACP_TOKEN")
	apiKey := os.Getenv("HERMES_API_KEY")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	// ── 2. Initialize store ──────────────────────────────────
	cfg := store.StoreConfig{Driver: "postgres", URL: dbURL}
	dataStore, err := store.NewStore(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("init postgres store: %w", err)
	}
	defer dataStore.Close()

	poolProvider, ok := dataStore.(store.PoolProvider)
	if !ok {
		return fmt.Errorf("store driver does not support pool access (got %T)", dataStore)
	}
	pool := poolProvider.Pool()
	_ = pool // used by gateway runner and chat handler below

	// pgStore aliases dataStore for backward compat with .Tenants()/.APIKeys() calls.
	pgStore := dataStore

	// ── 3. Seed default tenant (for static token auth) ────────
	if acpToken != "" {
		if err := seedDefaultTenant(context.Background(), pgStore); err != nil {
			slog.Warn("failed to seed default tenant", "error", err)
		}
	}

	// ── 4. Build auth chain ──────────────────────────────────
	authChain := auth.NewExtractorChain()

	// Static token extractor (for backward compatibility and dev mode).
	if acpToken != "" {
		authChain.Add(auth.NewStaticTokenExtractor(acpToken, "00000000-0000-0000-0000-000000000001"))
	}

	// API key extractor.
	authChain.Add(auth.NewAPIKeyExtractor(pgStore.APIKeys()))

	// JWT extractor — add JWT config here in production.
	// authChain.Extractors = append(authChain.Extractors, auth.NewJWTExtractor(...))

	// ── 4. RBAC config ──────────────────────────────────────
	rbacCfg := middleware.RBACConfig{
		DefaultRole: "user",
		Rules: map[string]string{
			"/v1/tenants":    "admin",
			"/v1/tenants/":   "admin",
			"/v1/api-keys":   "admin",
			"/v1/api-keys/":  "admin",
			"/v1/audit-logs": "admin",
			"/v1/gdpr/":      "admin",
		},
	}

	// ── 5. Rate limit config ─────────────────────────────────
	rateLimitCfg := middleware.RateLimitConfig{
		DefaultRPM: 60,
		TenantLimitFn: func(tenantID string) int {
			t, err := pgStore.Tenants().Get(context.Background(), tenantID)
			if err != nil || t == nil {
				return 0 // fall back to DefaultRPM
			}
			return t.RateLimitRPM
		},
	}

	// Inject distributed Redis rate limiter if REDIS_URL is configured.
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		rc, err := rediscache.New(context.Background(), redisURL)
		if err != nil {
			slog.Warn("Redis rate limiter unavailable, using local fallback", "error", err)
		} else {
			rateLimitCfg.Limiter = rc
			slog.Info("Distributed Redis rate limiter enabled")
		}
	}

	// ── 6. Initialize MinIO client for per-tenant skills and soul storage ──
	var skillsClient *objstore.MinIOClient
	if minioEndpoint := os.Getenv("MINIO_ENDPOINT"); minioEndpoint != "" {
		minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
		minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
		minioBucket := os.Getenv("MINIO_BUCKET")
		if minioAccessKey != "" && minioSecretKey != "" && minioBucket != "" {
			skillsClient, err = objstore.NewMinIOClient(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, false)
			if err != nil {
				slog.Warn("minio_client_init_failed", "endpoint", minioEndpoint, "error", err)
				skillsClient = nil
			} else {
				if ensureErr := skillsClient.EnsureBucket(context.Background()); ensureErr != nil {
					slog.Warn("minio_bucket_init_failed", "bucket", minioBucket, "error", ensureErr)
				} else {
					slog.Info("MinIO client initialized", "endpoint", minioEndpoint, "bucket", minioBucket)
				}
			}
		}
	}

	// ── 7. Tenant provisioner (soul + skills) ────────────────
	var tenantOpts []api.TenantHandlerOption
	if skillsClient != nil {
		prov := skills.NewProvisioner(skillsClient, "skills")
		tenantOpts = append(tenantOpts, api.WithOnTenantCreated(func(ctx context.Context, tenantID string) {
			if err := prov.Provision(ctx, tenantID); err != nil {
				slog.Error("tenant provisioning failed", "tenant", tenantID, "error", err)
			}
		}))

	}

	// Provisioner reference for lifecycle-managed background sync.
	var syncProv *skills.Provisioner
	if skillsClient != nil {
		syncProv = skills.NewProvisioner(skillsClient, "skills")
	}

	// ── 8. Build API server config ───────────────────────────
	serverCfg := api.APIServerConfig{
		Port:           port,
		Store:          dataStore,
		DB:             pool,
		Pool:           pool,
		AuthChain:      authChain,
		RBAC:           rbacCfg,
		RateLimit:      rateLimitCfg,
		AllowedOrigins: allowedOrigins,
		StaticDir:      staticDir,
		SkillsClient:   skillsClient,
		TenantOpts:     tenantOpts,
	}

	saasServer := api.NewAPIServer(serverCfg)

	// ── 9. Optionally prepare ACP server ─────────────────────
	var acpServer *acp.ACPServer
	if acpPortStr := os.Getenv("HERMES_ACP_PORT"); acpPortStr != "" {
		if acpPort, err := strconv.Atoi(acpPortStr); err == nil && acpPort > 0 {
			acpServer = acp.NewACPServer(acpPort)
		}
	}

	// ── 10. Optionally prepare gateway runner ────────────────
	var runner *gateway.Runner
	if apiKey != "" {
		adapterPortStr := os.Getenv("HERMES_API_PORT")
		if adapterPortStr == "" {
			adapterPortStr = "8081"
		}
		if adapterPort, err := strconv.Atoi(adapterPortStr); err == nil && adapterPort > 0 {
			gwCfg := gateway.DefaultGatewayConfig()
			gwCfg.AllowedUsers = map[string]any{
				"api": []any{"*"},
			}
			runner = gateway.NewRunner(gwCfg, pool)

			adapter := platforms.NewAPIServerAdapter(adapterPort, apiKey)
			runner.RegisterAdapter(adapter)
		}
	}

	// ── 11. Lifecycle: start all services, signal handling, ordered shutdown ─
	// Start background services in goroutines.
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()

	if syncProv != nil {
		go func() {
			if err := syncProv.SyncAllTenants(syncCtx, pgStore.Tenants()); err != nil && syncCtx.Err() == nil {
				slog.Error("startup tenant sync failed", "error", err)
			}
		}()
	}

	if acpServer != nil {
		go func() {
			slog.Info("ACP server starting", "port", os.Getenv("HERMES_ACP_PORT"))
			if err := acpServer.Start(); err != nil {
				slog.Warn("ACP server error", "error", err)
			}
		}()
	}

	if runner != nil {
		go func() {
			slog.Info("Gateway runner starting")
			if err := runner.Start(); err != nil {
				slog.Error("Gateway runner error", "error", err)
			}
		}()
	}

	// Signal handler with ordered shutdown (LIFO: ingress → processing → storage).
	done := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("Shutting down (grace period 15s)...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		// 1. Stop accepting new requests.
		if err := saasServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("API server shutdown error", "error", err)
		}
		// 2. Stop gateway runner (drains in-flight agent conversations).
		if runner != nil {
			runner.Stop()
		}
		// 3. Stop ACP server.
		if acpServer != nil {
			_ = acpServer.Stop()
		}
		// 4. Cancel background sync.
		syncCancel()
		// 5. Close data store.
		_ = dataStore.Close()

		slog.Info("Shutdown complete")
		close(done)
	}()

	slog.Info("SaaS API server running",
		"port", port,
		"openapi", fmt.Sprintf("http://localhost:%d/v1/openapi", port),
		"admin", fmt.Sprintf("http://localhost:%d/admin.html", port),
		"health_live", fmt.Sprintf("http://localhost:%d/health/live", port),
		"health_ready", fmt.Sprintf("http://localhost:%d/health/ready", port),
	)

	err = saasServer.Start()
	<-done
	return err
}

// seedDefaultTenant creates the default SaaS tenant if it does not already exist.
// It is idempotent — calling it multiple times is safe.
func seedDefaultTenant(ctx context.Context, pgStore store.Store) error {
	const defaultTenantID = "00000000-0000-0000-0000-000000000001"

	_, err := pgStore.Tenants().Get(ctx, defaultTenantID)
	if err == nil {
		// Tenant already exists.
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("seedDefaultTenant: get tenant: %w", err)
	}

	// Create the default tenant.
	defaultTenant := &store.Tenant{
		ID:           defaultTenantID,
		Name:         "Default Tenant",
		Plan:         "pro",
		RateLimitRPM: 120,
		MaxSessions:  10,
	}
	if err := pgStore.Tenants().Create(ctx, defaultTenant); err != nil {
		return fmt.Errorf("seedDefaultTenant: create tenant: %w", err)
	}

	slog.Info("seeded default tenant", "id", defaultTenantID)
	return nil
}
