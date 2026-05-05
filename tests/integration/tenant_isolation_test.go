//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

func TestTenantA_CannotAccessTenantB_Sessions(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "isolation-a-sess", "pro")
	tenantB := testEnv.CreateTestTenant(t, "isolation-b-sess", "pro")

	ctx := context.Background()
	// Create a session belonging to tenant B
	sessB := &store.Session{
		ID:        fmt.Sprintf("sess-b-%d", time.Now().UnixNano()),
		Platform:  "api",
		UserID:    "user-b",
		Model:     "test-model",
		StartedAt: time.Now(),
	}
	if err := testEnv.Store.Sessions().Create(ctx, tenantB.ID, sessB); err != nil {
		t.Fatalf("create session B: %v", err)
	}

	// Tenant A tries to access Tenant B's session via API
	resp := testEnv.DoRequest(t, "GET", "/v1/sessions/"+sessB.ID, "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)

	// Should not return tenant B's session data with actual messages
	if resp.StatusCode == http.StatusOK {
		var result map[string]any
		json.Unmarshal([]byte(body), &result)
		// Check that no messages are leaked (empty array or nil is acceptable)
		if msgs, ok := result["messages"]; ok {
			if msgArr, isArr := msgs.([]any); isArr && len(msgArr) > 0 {
				t.Errorf("tenant A should not see tenant B's session messages, got: %s", body)
			}
		}
		// Also verify the tenant_id in response matches tenant A (not B)
		if tid, ok := result["tenant_id"].(string); ok && tid == tenantB.ID {
			t.Errorf("response leaks tenant B's ID: %s", body)
		}
	}
}

func TestTenantA_CannotAccessTenantB_Messages(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "isolation-a-msg", "pro")
	tenantB := testEnv.CreateTestTenant(t, "isolation-b-msg", "pro")

	ctx := context.Background()
	sessID := fmt.Sprintf("shared-sess-%d", time.Now().UnixNano())

	// Create session and message for tenant B
	sessB := &store.Session{ID: sessID, Platform: "api", UserID: "user-b", Model: "test-model", StartedAt: time.Now()}
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, sessB)
	testEnv.Store.Messages().Append(ctx, tenantB.ID, sessID, &store.Message{
		Role: "user", Content: "secret-message-from-tenant-b", Timestamp: time.Now(),
	})

	// Tenant A queries messages for that session ID
	resp := testEnv.DoRequest(t, "GET", "/v1/sessions/"+sessID, "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)

	if containsString(body, "secret-message-from-tenant-b") {
		t.Errorf("tenant A can see tenant B's messages: %s", body)
	}
}

func TestTenantHeader_Ignored(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "header-ignore-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "header-ignore-b", "pro")

	// Tenant A sends request with X-Tenant-ID header set to tenant B
	headers := map[string]string{"X-Tenant-ID": tenantB.ID}
	resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", tenantA.APIKey, headers)
	body := ReadBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Even with the header, the response should only contain tenant A's data
	// (which is empty since no sessions created for A)
	var result map[string]any
	json.Unmarshal([]byte(body), &result)
	// Should not contain any sessions from tenant B
	if containsString(body, tenantB.ID) {
		t.Errorf("X-Tenant-ID header should be ignored, but response contains tenant B data: %s", body)
	}
}

func TestInvalidTenantID_Rejected(t *testing.T) {
	// Create a tenant with a valid key, then try to craft requests
	// The middleware validates tenant ID format via regex
	// Since tenant ID comes from the API key (not header), we test at the store level
	ctx := context.Background()

	// Attempt to create tenant with path traversal ID should fail at DB level
	badTenant := &store.Tenant{ID: "../escape", Name: "bad", Plan: "free", RateLimitRPM: 10, MaxSessions: 5}
	err := testEnv.Store.Tenants().Create(ctx, badTenant)
	if err == nil {
		t.Error("expected error creating tenant with path traversal ID, got nil")
		testEnv.cleanupTenant("../escape")
	}
}

func TestRLS_DirectQuery_CrossTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-b", "pro")

	ctx := context.Background()

	// Insert a session for each tenant
	sessA := &store.Session{ID: fmt.Sprintf("rls-sess-a-%d", time.Now().UnixNano()), Platform: "api", UserID: "u1", Model: "m", StartedAt: time.Now()}
	sessB := &store.Session{ID: fmt.Sprintf("rls-sess-b-%d", time.Now().UnixNano()), Platform: "api", UserID: "u1", Model: "m", StartedAt: time.Now()}
	testEnv.Store.Sessions().Create(ctx, tenantA.ID, sessA)
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, sessB)

	// Use RLS pool: set tenant to A inside a transaction, try to read B's session
	conn, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire rls conn: %v", err)
	}
	defer conn.Release()

	// SET LOCAL only works inside a transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	// Set RLS context to tenant A
	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
	if err != nil {
		t.Fatalf("set rls tenant: %v", err)
	}

	// Try to read tenant B's session — should return 0 rows
	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessB.ID).Scan(&count)
	if err != nil {
		t.Fatalf("rls query: %v", err)
	}
	if count != 0 {
		t.Errorf("RLS violation: tenant A context can see tenant B's session, count=%d", count)
	}

	// Verify tenant A can see their own session
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessA.ID).Scan(&count)
	if err != nil {
		t.Fatalf("rls query own: %v", err)
	}
	if count != 1 {
		t.Errorf("RLS: tenant A should see their own session, count=%d", count)
	}
}

func TestRLS_SessionVariable_Reset(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-reset-a", "pro")
	ctx := context.Background()

	// Set tenant variable on a connection
	conn, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	_, err = conn.Exec(ctx, fmt.Sprintf("SET app.current_tenant = '%s'", tenantA.ID))
	if err != nil {
		t.Fatalf("set tenant: %v", err)
	}
	conn.Release()

	// Acquire again — the main pool has RESET ALL in AfterRelease
	// For the RLS pool without that hook, verify the variable state
	conn2, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer conn2.Release()

	var val string
	err = conn2.QueryRow(ctx, "SELECT current_setting('app.current_tenant', true)").Scan(&val)
	if err != nil {
		t.Fatalf("read setting: %v", err)
	}
	// After pool reset, should be empty or default
	// Note: RLS pool may not have RESET ALL hook — this test documents the behavior
	t.Logf("app.current_tenant after release+reacquire: %q", val)
}

func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) > 0 && (haystack == needle || len(haystack) > len(needle) && findSubstring(haystack, needle))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
