//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/api"
	"github.com/hermes-agent/hermes-agent-go/internal/auth"
	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/objstore"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
	"github.com/hermes-agent/hermes-agent-go/internal/store/pg"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testEnv *TestEnv

type TestEnv struct {
	Pool     *pgxpool.Pool
	RLSPool  *pgxpool.Pool
	Store    store.Store
	MinIO    *objstore.MinIOClient
	Server   *httptest.Server
	MockLLM  *httptest.Server
	shutdown func()
}

type TestTenant struct {
	ID     string
	Name   string
	APIKey string // raw key (unhashed)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	env, err := SetupTestEnv(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
		os.Exit(1)
	}
	testEnv = env

	code := m.Run()

	testEnv.Teardown()
	os.Exit(code)
}

func SetupTestEnv(ctx context.Context) (*TestEnv, error) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://hermes_test:hermes_test@localhost:5433/hermes_test?sslmode=disable"
	}

	minioEndpoint := os.Getenv("TEST_MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "localhost:9002"
	}

	// Main pool (superuser for setup/teardown)
	pgStore, err := pg.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("pg store: %w", err)
	}
	if err := pgStore.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}

	// Create RLS test role (idempotent)
	rlsSetupSQL := `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'hermes_rls') THEN
				CREATE ROLE hermes_rls LOGIN PASSWORD 'hermes_rls';
			END IF;
		END $$;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO hermes_rls;
		GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO hermes_rls;
	`
	if _, err := pool.Exec(ctx, rlsSetupSQL); err != nil {
		return nil, fmt.Errorf("rls role setup: %w", err)
	}

	// RLS pool (restricted user)
	rlsURL := strings.Replace(dbURL, "hermes_test:hermes_test", "hermes_rls:hermes_rls", 1)
	rlsPool, err := pgxpool.New(ctx, rlsURL)
	if err != nil {
		return nil, fmt.Errorf("rls pool: %w", err)
	}

	// MinIO
	minioBucket := os.Getenv("TEST_MINIO_BUCKET")
	if minioBucket == "" {
		minioBucket = "hermes-test"
	}
	minioClient, err := objstore.NewMinIOClient(minioEndpoint, "hermes_test", "hermes_test", minioBucket, false)
	if err != nil {
		return nil, fmt.Errorf("minio: %w", err)
	}
	if err := minioClient.EnsureBucket(ctx); err != nil {
		return nil, fmt.Errorf("minio bucket: %w", err)
	}

	// Mock LLM
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "Test response from mock LLM"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))

	// Set LLM env vars for chat handler
	os.Setenv("LLM_API_URL", mockLLM.URL)
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test-model")
	os.Setenv("HERMES_BASE_URL", mockLLM.URL+"/v1")
	os.Setenv("HERMES_API_KEY_LLM", "test-key")
	os.Setenv("HERMES_MODEL", "test-model")
	os.Setenv("HERMES_PROVIDER", "openai")

	// Auth chain using API key extractor backed by real PG store
	chain := auth.NewExtractorChain(
		auth.NewAPIKeyExtractor(pgStore.APIKeys()),
	)

	// Build API server
	apiSrv := api.NewAPIServer(api.APIServerConfig{
		Port:           0,
		Store:          pgStore,
		AuthChain:      chain,
		Pool:           pool,
		AllowedOrigins: "*",
		SkillsClient:   minioClient,
		RBAC: middleware.RBACConfig{
			DefaultRole: "user",
			Rules:       map[string]string{},
		},
		RateLimit: middleware.RateLimitConfig{
			DefaultRPM: 1000,
		},
	})

	// Use httptest.Server instead of real listener
	ts := httptest.NewServer(apiSrv.Handler())

	return &TestEnv{
		Pool:    pool,
		RLSPool: rlsPool,
		Store:   pgStore,
		MinIO:   minioClient,
		Server:  ts,
		MockLLM: mockLLM,
		shutdown: func() {
			ts.Close()
			mockLLM.Close()
			rlsPool.Close()
			pool.Close()
			pgStore.Close()
		},
	}, nil
}

func (e *TestEnv) Teardown() {
	if e.shutdown != nil {
		e.shutdown()
	}
}

// CreateTestTenant creates a tenant with a working API key. Returns the raw key for Authorization header.
func (e *TestEnv) CreateTestTenant(t *testing.T, name, plan string) *TestTenant {
	t.Helper()
	ctx := context.Background()

	tenant := &store.Tenant{Name: name, Plan: plan, RateLimitRPM: 1000, MaxSessions: 100}
	if err := e.Store.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant %s: %v", name, err)
	}

	// Generate a random API key
	rawKey := generateRawKey()
	hash := hashKey(rawKey)

	apiKey := &store.APIKey{
		TenantID: tenant.ID,
		Name:     name + "-key",
		KeyHash:  hash,
		Prefix:   rawKey[:8],
		Roles:    []string{"user", "admin"},
	}
	if err := e.Store.APIKeys().Create(ctx, apiKey); err != nil {
		t.Fatalf("create api key for %s: %v", name, err)
	}

	t.Cleanup(func() {
		e.cleanupTenant(tenant.ID)
	})

	return &TestTenant{ID: tenant.ID, Name: name, APIKey: rawKey}
}

func (e *TestEnv) cleanupTenant(tenantID string) {
	ctx := context.Background()
	// Clean in dependency order
	e.Pool.Exec(ctx, "DELETE FROM messages WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM sessions WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM memories WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM user_profiles WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM audit_logs WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM api_keys WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM roles WHERE tenant_id = $1", tenantID)
	e.Pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenantID)
}

// DoRequest sends an HTTP request to the test server with the given API key.
func (e *TestEnv) DoRequest(t *testing.T, method, path string, body string, apiKey string, extraHeaders map[string]string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, e.Server.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// ReadBody reads and closes the response body.
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

// SendChat sends a chat completion request and returns the response.
func (e *TestEnv) SendChat(t *testing.T, apiKey, sessionID, userID, message string) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"model":"test-model","messages":[{"role":"user","content":%q}]}`, message)
	headers := map[string]string{
		"X-Hermes-Session-Id": sessionID,
		"X-Hermes-User-Id":    userID,
	}
	return e.DoRequest(t, "POST", "/v1/chat/completions", body, apiKey, headers)
}

func generateRawKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "hk_" + hex.EncodeToString(b)
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
