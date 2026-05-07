//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestSession_TenantScoped_List(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sess-list-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sess-list-b", "pro")
	ctx := context.Background()

	// Create sessions for both tenants
	for i := 0; i < 3; i++ {
		testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
			ID: fmt.Sprintf("a-sess-%d-%d", i, time.Now().UnixNano()), Platform: "api", UserID: "ua", Model: "m", StartedAt: time.Now(),
		})
		testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
			ID: fmt.Sprintf("b-sess-%d-%d", i, time.Now().UnixNano()), Platform: "api", UserID: "ub", Model: "m", StartedAt: time.Now(),
		})
	}

	// List sessions for tenant A via API
	resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Should not contain any of tenant B's session IDs
	if containsString(body, "b-sess-") {
		t.Errorf("tenant A list contains tenant B sessions: %s", body)
	}
}

func TestSession_CrossTenant_Get_Returns_Nil(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sess-get-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sess-get-b", "pro")
	ctx := context.Background()

	sessID := fmt.Sprintf("cross-get-%d", time.Now().UnixNano())
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessID, Platform: "api", UserID: "ub", Model: "m", StartedAt: time.Now(),
	})

	// Tenant A tries to Get tenant B's session directly via store
	sess, err := testEnv.Store.Sessions().Get(ctx, tenantA.ID, sessID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess != nil {
		t.Errorf("tenant A should not be able to get tenant B's session, got: %+v", sess)
	}
}

func TestSession_Delete_OnlyOwnTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sess-del-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sess-del-b", "pro")
	ctx := context.Background()

	sessID := fmt.Sprintf("del-target-%d", time.Now().UnixNano())
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessID, Platform: "api", UserID: "ub", Model: "m", StartedAt: time.Now(),
	})

	// Tenant A tries to delete tenant B's session
	err := testEnv.Store.Sessions().Delete(ctx, tenantA.ID, sessID)
	if err != nil {
		t.Logf("delete returned error (expected): %v", err)
	}

	// Verify session still exists for tenant B
	sess, _ := testEnv.Store.Sessions().Get(ctx, tenantB.ID, sessID)
	if sess == nil {
		t.Error("tenant B's session was deleted by tenant A — isolation failure")
	}
}

func TestSession_Messages_TenantScoped(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sess-msg-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sess-msg-b", "pro")
	ctx := context.Background()

	sessID := fmt.Sprintf("msg-scope-%d", time.Now().UnixNano())

	// Both tenants create sessions with the same ID (different namespaces)
	testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
		ID: sessID, Platform: "api", UserID: "ua", Model: "m", StartedAt: time.Now(),
	})
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessID, Platform: "api", UserID: "ub", Model: "m", StartedAt: time.Now(),
	})

	// Append messages to each tenant's session
	testEnv.Store.Messages().Append(ctx, tenantA.ID, sessID, &store.Message{
		Role: "user", Content: "message-from-tenant-a", Timestamp: time.Now(),
	})
	testEnv.Store.Messages().Append(ctx, tenantB.ID, sessID, &store.Message{
		Role: "user", Content: "message-from-tenant-b", Timestamp: time.Now(),
	})

	// List messages for tenant A — should only see A's messages
	msgsA, err := testEnv.Store.Messages().List(ctx, tenantA.ID, sessID, 50, 0)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	for _, m := range msgsA {
		if m.Content == "message-from-tenant-b" {
			t.Error("tenant A can see tenant B's message")
		}
	}

	// List messages for tenant B — should only see B's messages
	msgsB, err := testEnv.Store.Messages().List(ctx, tenantB.ID, sessID, 50, 0)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	for _, m := range msgsB {
		if m.Content == "message-from-tenant-a" {
			t.Error("tenant B can see tenant A's message")
		}
	}
}

func TestSession_Concurrent_Tenants(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sess-conc-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sess-conc-b", "pro")
	ctx := context.Background()

	const iterations = 20
	var wg sync.WaitGroup
	errors := make(chan string, iterations*2)

	// Concurrently create sessions and messages for both tenants
	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			sessID := fmt.Sprintf("conc-a-%d-%d", idx, time.Now().UnixNano())
			testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
				ID: sessID, Platform: "api", UserID: "ua", Model: "m", StartedAt: time.Now(),
			})
			testEnv.Store.Messages().Append(ctx, tenantA.ID, sessID, &store.Message{
				Role: "user", Content: fmt.Sprintf("a-msg-%d", idx), Timestamp: time.Now(),
			})
		}(i)
		go func(idx int) {
			defer wg.Done()
			sessID := fmt.Sprintf("conc-b-%d-%d", idx, time.Now().UnixNano())
			testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
				ID: sessID, Platform: "api", UserID: "ub", Model: "m", StartedAt: time.Now(),
			})
			testEnv.Store.Messages().Append(ctx, tenantB.ID, sessID, &store.Message{
				Role: "user", Content: fmt.Sprintf("b-msg-%d", idx), Timestamp: time.Now(),
			})
		}(i)
	}
	wg.Wait()
	close(errors)

	// Verify no cross-contamination: list all sessions for each tenant
	sessionsA, _, _ := testEnv.Store.Sessions().List(ctx, tenantA.ID, store.ListOptions{Limit: 100})
	sessionsB, _, _ := testEnv.Store.Sessions().List(ctx, tenantB.ID, store.ListOptions{Limit: 100})

	for _, s := range sessionsA {
		if s.TenantID != "" && s.TenantID != tenantA.ID {
			t.Errorf("tenant A list contains wrong tenant: %s", s.TenantID)
		}
	}
	for _, s := range sessionsB {
		if s.TenantID != "" && s.TenantID != tenantB.ID {
			t.Errorf("tenant B list contains wrong tenant: %s", s.TenantID)
		}
	}

	// Verify message counts
	var result struct {
		Sessions []json.RawMessage `json:"sessions"`
	}
	resp := testEnv.DoRequest(t, "GET", "/v1/sessions", "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)
	json.Unmarshal([]byte(body), &result)

	if !containsString(body, "conc-a-") {
		t.Logf("tenant A sessions response: %s", body[:min(len(body), 500)])
	}
	if containsString(body, "conc-b-") {
		t.Error("tenant A response contains tenant B's concurrent sessions")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
