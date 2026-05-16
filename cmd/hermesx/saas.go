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

	"github.com/Colin4k1024/hermesx/internal/acp"
	"github.com/Colin4k1024/hermesx/internal/api"
	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/gateway"
	"github.com/Colin4k1024/hermesx/internal/gateway/platforms"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/observability"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	_ "github.com/Colin4k1024/hermesx/internal/store/mysql"
	pgstore "github.com/Colin4k1024/hermesx/internal/store/pg"
	"github.com/Colin4k1024/hermesx/internal/store/rediscache"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
  HERMES_BOOTSTRAP_RATE_LIMIT_RPM  IP-level bootstrap attempts per minute (default: 5)
  HERMES_API_PORT         OpenAI-compatible adapter port (default: 8081)
  HERMES_API_KEY          Bearer token for OpenAI-compatible adapter
  HERMES_ACP_PORT         ACP server port

Examples:
  hermesx saas-api                                           # Basic
  SAAS_API_PORT=8080 SAAS_STATIC_DIR=./internal/dashboard/static hermesx saas-api
`,
	RunE: runSaaSAPI,
}

func init() {
	rootCmd.AddCommand(saasAPICmd)
}

func runSaaSAPI(cmd *cobra.Command, args []string) error {
	setupLogging()

	// ── 0a. pprof admin server (env-gated, production needs IP allowlist) ──
	if adminPort := os.Getenv("HERMESX_ADMIN_PORT"); adminPort != "" {
		go api.StartAdminServer(adminPort)
	}

	// ── 0. OTel tracing (no-op if OTEL_EXPORTER_OTLP_ENDPOINT unset) ──
	tracerShutdown, err := observability.InitTracer(context.Background(), "hermesx", version)
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
	bootstrapRateLimitRPM := 5
	if v := os.Getenv("HERMES_BOOTSTRAP_RATE_LIMIT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			bootstrapRateLimitRPM = n
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	// ── 2. Initialize store ──────────────────────────────────
	// DATABASE_DRIVER selects the backend: "postgres" (default) or "mysql".
	// For MySQL, DATABASE_URL must use go-sql-driver DSN format:
	//   user:pass@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4
	driver := os.Getenv("DATABASE_DRIVER")
	if driver == "" {
		driver = "postgres"
	}
	cfg := store.StoreConfig{Driver: driver, URL: dbURL}
	dataStore, err := store.NewStore(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("init %s store: %w", driver, err)
	}
	defer dataStore.Close()

	// Pool access is PostgreSQL-specific; optional for health check and gateway sessions.
	var pool *pgxpool.Pool
	if poolProvider, ok := dataStore.(pgstore.PoolProvider); ok {
		pool = poolProvider.Pool()
		slog.Info("PostgreSQL pool available", "driver", driver)
	} else {
		slog.Info("Store initialized without direct pool access", "driver", driver)
	}

	// Resolve DBPinger for readiness health check (PG pool or store.Ping if available).
	var dbPinger api.DBPinger
	if pool != nil {
		dbPinger = pool
	} else if pinger, ok := dataStore.(api.DBPinger); ok {
		dbPinger = pinger
	}

	// pgStore aliases dataStore for backward compat with .Tenants()/.APIKeys() calls.
	pgStore := dataStore

	// ── 3. Seed default tenant (for static token auth) ────────
	var defaultTenantJustSeeded bool
	if acpToken != "" {
		var seedErr error
		defaultTenantJustSeeded, seedErr = seedDefaultTenant(context.Background(), pgStore)
		if seedErr != nil {
			slog.Warn("failed to seed default tenant", "error", seedErr)
		}
	}

	// ── 4. Build auth chain ──────────────────────────────────
	authChain := auth.NewExtractorChain()

	// Static token extractor (for backward compatibility and dev mode).
	if acpToken != "" {
		authChain.Add(auth.NewStaticTokenExtractor(acpToken, "00000000-0000-0000-0000-000000000001"))
	}

	// OIDC extractor (activated when OIDC_ISSUER_URL is set).
	if oidcIssuer := os.Getenv("OIDC_ISSUER_URL"); oidcIssuer != "" {
		oidcClientID := os.Getenv("OIDC_CLIENT_ID")
		if oidcClientID == "" {
			return fmt.Errorf("OIDC_ISSUER_URL is set but OIDC_CLIENT_ID is missing")
		}
		mapper := &auth.ClaimMapper{
			TenantClaim: os.Getenv("OIDC_TENANT_CLAIM"),
			RolesClaim:  os.Getenv("OIDC_ROLES_CLAIM"),
			ACRClaim:    os.Getenv("OIDC_ACR_CLAIM"),
		}
		discCtx, discCancel := context.WithTimeout(context.Background(), 15*time.Second)
		oidcExtractor, err := auth.NewOIDCExtractor(discCtx, auth.OIDCConfig{
			IssuerURL:   oidcIssuer,
			ClientID:    oidcClientID,
			ClaimMapper: mapper,
		})
		discCancel()
		if err != nil {
			return fmt.Errorf("oidc extractor init: %w", err)
		}
		authChain.Add(oidcExtractor)
		slog.Info("OIDC extractor enabled", "issuer", oidcIssuer, "client_id", oidcClientID)
	}

	// API key extractor.
	authChain.Add(auth.NewAPIKeyExtractor(pgStore.APIKeys()))

	// ── 4. RBAC config ──────────────────────────────────────
	rbacCfg := middleware.RBACConfig{
		DefaultRole: "user",
		Rules: map[string]string{
			"/v1/tenants":            "admin",
			"/v1/tenants/":           "admin",
			"/v1/api-keys":           "admin",
			"/v1/api-keys/":          "admin",
			"GET /v1/audit-logs":     "auditor",
			"/v1/gdpr/":              "admin",
			"/v1/execution-receipts": "auditor",
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

	// ── 6. Initialize object store client (MinIO / RustFS) for per-tenant skills ──
	var skillsClient objstore.ObjectStore
	if minioEndpoint := os.Getenv("MINIO_ENDPOINT"); minioEndpoint != "" {
		minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
		minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
		minioBucket := os.Getenv("MINIO_BUCKET")
		if minioAccessKey != "" && minioSecretKey != "" && minioBucket != "" {
			skillsClient, err = objstore.NewObjStoreClient(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, false)
			if err != nil {
				slog.Warn("objstore_client_init_failed", "endpoint", minioEndpoint, "error", err)
				skillsClient = nil
			} else {
				if ensureErr := skillsClient.EnsureBucket(context.Background()); ensureErr != nil {
					slog.Warn("objstore_bucket_init_failed", "bucket", minioBucket, "error", ensureErr)
				} else {
					slog.Info("objstore client initialized", "endpoint", minioEndpoint, "bucket", minioBucket)
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

	// ── 7.5. Provision default tenant immediately if just seeded ──
	if defaultTenantJustSeeded && syncProv != nil {
		const defaultTenantID = "00000000-0000-0000-0000-000000000001"
		if err := syncProv.Provision(context.Background(), defaultTenantID); err != nil {
			slog.Warn("default tenant initial provisioning failed", "error", err)
		}
	}

	// ── 8. Build API server config ───────────────────────────
	serverCfg := api.APIServerConfig{
		Port:                  port,
		Store:                 dataStore,
		DB:                    dbPinger,
		AuthChain:             authChain,
		RBAC:                  rbacCfg,
		RateLimit:             rateLimitCfg,
		BootstrapRateLimitRPM: bootstrapRateLimitRPM,
		AllowedOrigins:        allowedOrigins,
		StaticDir:             staticDir,
		SkillsClient:          skillsClient,
		Provisioner:           syncProv,
		TenantOpts:            tenantOpts,
	}

	saasServer := api.NewAPIServer(serverCfg)

	// ── 8.5. Wire Oris evolution (optional, global shared store) ──────────
	var evolutionStore *evolution.GeneStore // lifted to function scope for shutdown (B3)
	{
		hermesCfg := config.Load()
		evCfg := evolution.Config{
			Enabled:          hermesCfg.Evolution.Enabled,
			StorageMode:      hermesCfg.Evolution.StorageMode,
			DBPath:           hermesCfg.Evolution.DBPath,
			MySQLDSN:         hermesCfg.Evolution.MySQLDSN,
			MinConfidence:    hermesCfg.Evolution.MinConfidence,
			ReplayThreshold:  hermesCfg.Evolution.ReplayThreshold,
			MaxGenesInPrompt: hermesCfg.Evolution.MaxGenesInPrompt,
		}
		if evCfg.Enabled {
			gs, evErr := evolution.Open(evCfg)
			if evErr != nil {
				slog.Warn("evolution store init failed, running without evolution", "error", evErr)
			} else {
				evolutionStore = gs
				evImp := evolution.NewImprover(gs, nil, evCfg)
				saasServer.AgentChat.SetEvolutionImprover(evImp)
				slog.Info("Oris evolution enabled (global shared store)")
			}
		}
	}

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
			runner = gateway.NewRunner(gwCfg, pool, dataStore)

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
		// 5. Close evolution store (flushes SQLite WAL / MySQL pool) (B3).
		if evolutionStore != nil {
			_ = evolutionStore.Close()
		}
		// 6. Close data store.
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
// Returns (true, nil) when the tenant was just created, (false, nil) when it already existed.
// Handles both MySQL (nil, nil for not-found) and PostgreSQL (nil, wrapped pgx.ErrNoRows) backends.
func seedDefaultTenant(ctx context.Context, pgStore store.Store) (bool, error) {
	const defaultTenantID = "00000000-0000-0000-0000-000000000001"

	t, err := pgStore.Tenants().Get(ctx, defaultTenantID)
	if err != nil {
		// PG wraps pgx.ErrNoRows as "not found"; MySQL returns nil,nil for not-found.
		// Any other error is a real failure.
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("seedDefaultTenant: get tenant: %w", err)
		}
		// ErrNoRows: tenant doesn't exist, fall through to create.
	}
	if t != nil {
		// Tenant already exists.
		return false, nil
	}
	// t is nil: either MySQL "not found" (nil,nil) or PG ErrNoRows.

	// Create the default tenant.
	defaultTenant := &store.Tenant{
		ID:           defaultTenantID,
		Name:         "Default Tenant",
		Plan:         "pro",
		RateLimitRPM: 120,
		MaxSessions:  10,
	}
	if err := pgStore.Tenants().Create(ctx, defaultTenant); err != nil {
		return false, fmt.Errorf("seedDefaultTenant: create tenant: %w", err)
	}

	slog.Info("seeded default tenant", "id", defaultTenantID)
	return true, nil
}
