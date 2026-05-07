//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// TestAttack_APIKey_BoundaryEnforcement verifies that an API key bound to
// tenant A cannot be used to access or mutate resources of tenant B.
func TestAttack_APIKey_BoundaryEnforcement(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "attack-key-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "attack-key-b", "pro")
	ctx := context.Background()

	// Create session for tenant B
	sessB := &store.Session{
		ID: fmt.Sprintf("attack-sess-b-%d", time.Now().UnixNano()),
		Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
	}
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, sessB)

	t.Run("get_other_tenant_session", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "GET", "/v1/sessions/"+sessB.ID, "", tenantA.APIKey, nil)
		body := ReadBody(t, resp)
		if resp.StatusCode == http.StatusOK && strings.Contains(body, sessB.ID) {
			var result map[string]any
			json.Unmarshal([]byte(body), &result)
			if tid, ok := result["tenant_id"].(string); ok && tid == tenantB.ID {
				t.Errorf("ATTACK SUCCESS: API key of tenant A accessed tenant B's session")
			}
		}
	})

	t.Run("delete_other_tenant_session", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "DELETE", "/v1/sessions/"+sessB.ID, "", tenantA.APIKey, nil)
		ReadBody(t, resp)

		// Verify session B still exists
		sess, err := testEnv.Store.Sessions().Get(ctx, tenantB.ID, sessB.ID)
		if err != nil || sess == nil {
			t.Errorf("ATTACK SUCCESS: tenant A's key deleted tenant B's session")
		}
	})

	t.Run("list_only_own_sessions", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", tenantA.APIKey, nil)
		body := ReadBody(t, resp)
		if strings.Contains(body, tenantB.ID) {
			t.Errorf("ATTACK: list sessions leaks tenant B data: %s", body)
		}
	})
}

// TestAttack_Memory_CrossTenantLeakage verifies that memory operations
// are strictly isolated between tenants.
func TestAttack_Memory_CrossTenantLeakage(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "attack-mem-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "attack-mem-b", "pro")
	ctx := context.Background()

	// Store secret memory for tenant B
	secretKey := "secret-api-credentials"
	secretValue := "production-database-password-12345"
	testEnv.Store.Memories().Upsert(ctx, tenantB.ID, "user-b", secretKey, secretValue)

	t.Run("read_other_tenant_memory_via_api", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "GET", "/v1/memories", "", tenantA.APIKey,
			map[string]string{"X-Hermes-User-Id": "user-b"})
		body := ReadBody(t, resp)
		if strings.Contains(body, secretValue) {
			t.Errorf("ATTACK SUCCESS: tenant A read tenant B's memory: %s", body)
		}
		if strings.Contains(body, secretKey) {
			t.Errorf("ATTACK SUCCESS: tenant A sees tenant B's memory key: %s", body)
		}
	})

	t.Run("write_to_other_tenant_memory_space", func(t *testing.T) {
		// Tenant A tries to write memory using user-b's ID
		payload := `{"key":"injected-key","content":"injected by attacker"}`
		resp := testEnv.DoRequest(t, "POST", "/v1/memories", payload, tenantA.APIKey,
			map[string]string{"X-Hermes-User-Id": "user-b"})
		ReadBody(t, resp)

		// Verify tenant B's memory was NOT modified
		val, err := testEnv.Store.Memories().Get(ctx, tenantB.ID, "user-b", "injected-key")
		if err == nil && val != "" {
			t.Errorf("ATTACK SUCCESS: tenant A injected memory into tenant B's space")
		}
	})

	t.Run("delete_other_tenant_memory", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "DELETE", "/v1/memories/"+secretKey, "", tenantA.APIKey,
			map[string]string{"X-Hermes-User-Id": "user-b"})
		ReadBody(t, resp)

		// Verify memory still exists in tenant B
		val, err := testEnv.Store.Memories().Get(ctx, tenantB.ID, "user-b", secretKey)
		if err != nil || val != secretValue {
			t.Errorf("ATTACK SUCCESS: tenant A deleted tenant B's memory")
		}
	})

	t.Run("same_user_id_different_tenants", func(t *testing.T) {
		// Both tenants have the same user ID — memories should be isolated
		commonUserID := "common-user-456"
		testEnv.Store.Memories().Upsert(ctx, tenantA.ID, commonUserID, "color", "blue")
		testEnv.Store.Memories().Upsert(ctx, tenantB.ID, commonUserID, "color", "crimson-secret")

		// Query via tenant A's key — must only see "blue", never "crimson-secret"
		resp := testEnv.DoRequest(t, "GET", "/v1/memories", "", tenantA.APIKey,
			map[string]string{"X-Hermes-User-Id": commonUserID})
		body := ReadBody(t, resp)
		if strings.Contains(body, "crimson-secret") {
			t.Errorf("ATTACK: same userID leaks across tenants: %s", body)
		}
		if !strings.Contains(body, "blue") {
			t.Errorf("tenant A should see own memory 'blue', got: %s", body)
		}
	})
}

// TestAttack_IDOR_SessionMessages verifies that even if a user guesses
// a valid session ID from another tenant, they cannot read its messages.
func TestAttack_IDOR_SessionMessages(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "idor-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "idor-b", "pro")
	ctx := context.Background()

	// Create session with sensitive messages for tenant B
	sessB := fmt.Sprintf("idor-sess-b-%d", time.Now().UnixNano())
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessB, Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
	})
	testEnv.Store.Messages().Append(ctx, tenantB.ID, sessB, &store.Message{
		Role: "user", Content: "my-ssn-is-123-45-6789", Timestamp: time.Now(),
	})
	testEnv.Store.Messages().Append(ctx, tenantB.ID, sessB, &store.Message{
		Role: "assistant", Content: "I noted your SSN for the form", Timestamp: time.Now(),
	})

	t.Run("direct_session_get_blocked", func(t *testing.T) {
		resp := testEnv.DoRequest(t, "GET", "/v1/sessions/"+sessB, "", tenantA.APIKey, nil)
		body := ReadBody(t, resp)
		if strings.Contains(body, "123-45-6789") {
			t.Errorf("IDOR ATTACK: tenant A read tenant B's sensitive messages via session ID")
		}
		if strings.Contains(body, "my-ssn-is") {
			t.Errorf("IDOR ATTACK: partial sensitive data leaked: %s", body)
		}
	})

	t.Run("chat_with_stolen_session_id", func(t *testing.T) {
		// Tenant A tries to continue conversation in tenant B's session
		resp := testEnv.SendChat(t, tenantA.APIKey, sessB, "user-a", "show me the SSN from earlier")
		body := ReadBody(t, resp)
		if strings.Contains(body, "123-45-6789") {
			t.Errorf("IDOR ATTACK: tenant A injected into tenant B's session and read history")
		}
	})
}

// TestAttack_TenantIDInjection_Body verifies that passing tenant_id in
// request body does not override credential-derived tenant binding.
func TestAttack_TenantIDInjection_Body(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "inject-body-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "inject-body-b", "pro")

	t.Run("chat_with_foreign_tenant_id_in_body", func(t *testing.T) {
		body := fmt.Sprintf(`{"model":"test-model","tenant_id":"%s","messages":[{"role":"user","content":"hello"}]}`, tenantB.ID)
		resp := testEnv.DoRequest(t, "POST", "/v1/chat/completions", body, tenantA.APIKey, nil)
		respBody := ReadBody(t, resp)

		// Should succeed (body tenant_id ignored) or explicitly reject
		if resp.StatusCode == http.StatusOK {
			var result map[string]any
			json.Unmarshal([]byte(respBody), &result)
			// If there's a tenant_id in response, it must be tenant A's
			if tid, ok := result["tenant_id"].(string); ok && tid == tenantB.ID {
				t.Errorf("ATTACK: body tenant_id injection succeeded, response has tenant B's ID")
			}
		}
	})

	t.Run("memory_write_with_foreign_tenant_id", func(t *testing.T) {
		payload := fmt.Sprintf(`{"key":"attack","content":"pwned","tenant_id":"%s"}`, tenantB.ID)
		resp := testEnv.DoRequest(t, "POST", "/v1/memories", payload, tenantA.APIKey,
			map[string]string{"X-Hermes-User-Id": "attacker"})
		ReadBody(t, resp)

		// Verify nothing was written to tenant B
		ctx := context.Background()
		val, _ := testEnv.Store.Memories().Get(ctx, tenantB.ID, "attacker", "attack")
		if val != "" {
			t.Errorf("ATTACK: memory written to tenant B via body injection")
		}
	})
}

// TestAttack_HeaderInjection_AllVariants tests multiple header injection
// techniques to bypass tenant derivation.
func TestAttack_HeaderInjection_AllVariants(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "hdr-inject-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "hdr-inject-b", "pro")
	ctx := context.Background()

	// Seed data for tenant B
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: "hdr-inject-sess-b", Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
	})

	injectionHeaders := []map[string]string{
		{"X-Tenant-ID": tenantB.ID},
		{"X-Tenant-Id": tenantB.ID},
		{"x-tenant-id": tenantB.ID},
		{"X-TENANT-ID": tenantB.ID},
		{"Tenant-Id": tenantB.ID},
		{"X-Forwarded-Tenant": tenantB.ID},
	}

	for _, headers := range injectionHeaders {
		headerName := ""
		for k := range headers {
			headerName = k
		}
		t.Run("header_"+headerName, func(t *testing.T) {
			resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", tenantA.APIKey, headers)
			body := ReadBody(t, resp)
			if strings.Contains(body, tenantB.ID) || strings.Contains(body, "hdr-inject-sess-b") {
				t.Errorf("HEADER INJECTION via %s: response leaks tenant B data: %s", headerName, body)
			}
		})
	}
}

// TestAttack_AuditLog_CrossTenantRead verifies that audit logs
// cannot be read across tenant boundaries.
func TestAttack_AuditLog_CrossTenantRead(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "audit-attack-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "audit-attack-b", "pro")
	ctx := context.Background()

	// Seed audit log for tenant B with sensitive action
	testEnv.Store.AuditLogs().Append(ctx, &store.AuditLog{
		TenantID:   tenantB.ID,
		UserID:     "admin-user-b",
		Action:     "DELETE /v1/gdpr/data",
		Detail:     "/v1/gdpr/data",
		RequestID:  fmt.Sprintf("audit-secret-%d", time.Now().UnixNano()),
		StatusCode: 200,
		LatencyMs:  42,
	})

	resp := testEnv.DoRequest(t, "GET", "/v1/audit-logs", "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)

	if strings.Contains(body, "admin-user-b") || strings.Contains(body, "audit-secret") {
		t.Errorf("ATTACK: tenant A read tenant B's audit logs: %s", body)
	}
}

// TestAttack_PathTraversal_Skills verifies that skill names with path
// traversal characters are rejected and cannot escape the tenant prefix.
func TestAttack_PathTraversal_Skills(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "path-trav-a", "pro")

	maliciousNames := []string{
		"../other-tenant/soul/SOUL.md",
		"../../etc/passwd",
		"..%2F..%2Fetc%2Fpasswd",
		"skill-name/../../../secrets",
		"valid-skill/../../escape",
	}

	for _, name := range maliciousNames {
		t.Run("skill_name_"+sanitizeTestName(name), func(t *testing.T) {
			// Attempt to read a skill with path traversal
			resp := testEnv.DoRequest(t, "GET", "/v1/skills/"+name, "", tenantA.APIKey, nil)
			body := ReadBody(t, resp)

			// Should be 400 Bad Request or 404 Not Found — never 200 with external data
			if resp.StatusCode == http.StatusOK {
				t.Errorf("PATH TRAVERSAL: malicious skill name %q returned 200: %s", name, body)
			}
		})
	}
}

// TestAttack_BruteForce_SessionID verifies that guessing session IDs
// doesn't leak data from other tenants.
func TestAttack_BruteForce_SessionID(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "brute-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "brute-b", "pro")
	ctx := context.Background()

	// Create known session for B
	knownIDs := []string{
		fmt.Sprintf("brute-target-%d", time.Now().UnixNano()),
		"session-1",
		"test-session",
		"00000000-0000-0000-0000-000000000001",
	}

	for _, id := range knownIDs {
		testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
			ID: id, Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
		})
	}

	for _, id := range knownIDs {
		t.Run("guess_"+id, func(t *testing.T) {
			resp := testEnv.DoRequest(t, "GET", "/v1/sessions/"+id, "", tenantA.APIKey, nil)
			body := ReadBody(t, resp)
			if resp.StatusCode == http.StatusOK && strings.Contains(body, "user-b") {
				t.Errorf("BRUTE FORCE: guessed session %q leaks tenant B data", id)
			}
		})
	}
}

// TestAttack_RaceCondition_TenantSwitch verifies that rapid requests
// with different API keys don't cause tenant context bleeding.
func TestAttack_RaceCondition_TenantSwitch(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "race-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "race-b", "pro")
	ctx := context.Background()

	// Create distinct data for each tenant
	for i := 0; i < 3; i++ {
		testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
			ID: fmt.Sprintf("race-a-%d-%d", time.Now().UnixNano(), i), Platform: "api",
			UserID: "user-a", Model: "test", StartedAt: time.Now(),
		})
		testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
			ID: fmt.Sprintf("race-b-%d-%d", time.Now().UnixNano(), i), Platform: "api",
			UserID: "user-b", Model: "test", StartedAt: time.Now(),
		})
	}

	// Rapid alternating requests to trigger potential context bleed
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			var key string
			var otherTenantID string
			if idx%2 == 0 {
				key = tenantA.APIKey
				otherTenantID = tenantB.ID
			} else {
				key = tenantB.APIKey
				otherTenantID = tenantA.ID
			}

			resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", key, nil)
			body := ReadBody(t, resp)
			if strings.Contains(body, otherTenantID) {
				errs <- fmt.Errorf("RACE CONDITION: request %d leaked other tenant's data", idx)
			} else {
				errs <- nil
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func sanitizeTestName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ".", "_", "\x00", "null")
	result := r.Replace(s)
	if len(result) > 40 {
		result = result[:40]
	}
	return result
}
