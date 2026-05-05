//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestUserA_CannotSeeUserB_Memory(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "user-mem-iso", "pro")
	ctx := context.Background()

	// User A stores a memory
	testEnv.Store.Memories().Upsert(ctx, tenant.ID, "user-alice", "secret", "alice-secret-data")

	// User B queries memories — should not see user A's data
	resp := testEnv.DoRequest(t, "GET", "/v1/memories", "", tenant.APIKey, map[string]string{
		"X-Hermes-User-Id": "user-bob",
	})
	body := ReadBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if containsString(body, "alice-secret-data") {
		t.Errorf("user-bob can see user-alice's memory: %s", body)
	}
}

func TestUserA_CannotSeeUserB_Profile(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "user-profile-iso", "pro")
	ctx := context.Background()

	// Set profile for user A
	testEnv.Store.UserProfiles().Upsert(ctx, tenant.ID, "profile-user-a", "I am user A with secret preferences")

	// User B queries /v1/me — should not see user A's profile
	resp := testEnv.DoRequest(t, "GET", "/v1/me", "", tenant.APIKey, map[string]string{
		"X-Hermes-User-Id": "profile-user-b",
	})
	body := ReadBody(t, resp)

	if containsString(body, "secret preferences") {
		t.Errorf("user B sees user A's profile: %s", body)
	}
}

func TestMemory_UniqueConstraint(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "mem-upsert", "pro")
	ctx := context.Background()

	// First write
	err := testEnv.Store.Memories().Upsert(ctx, tenant.ID, "upsert-user", "color", "blue")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second write with same key — should upsert, not error
	err = testEnv.Store.Memories().Upsert(ctx, tenant.ID, "upsert-user", "color", "red")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Verify latest value
	entries, err := testEnv.Store.Memories().List(ctx, tenant.ID, "upsert-user")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "color" {
			if e.Content != "red" {
				t.Errorf("expected 'red' after upsert, got %q", e.Content)
			}
			found = true
		}
	}
	if !found {
		t.Error("memory entry 'color' not found after upsert")
	}
}

func TestMemory_CrossTenant_SameUserID(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "mem-cross-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "mem-cross-b", "pro")
	ctx := context.Background()

	sharedUserID := "shared-user-123"

	// Store different memories for same userID in different tenants
	testEnv.Store.Memories().Upsert(ctx, tenantA.ID, sharedUserID, "secret", "tenant-a-secret")
	testEnv.Store.Memories().Upsert(ctx, tenantB.ID, sharedUserID, "secret", "tenant-b-secret")

	// Query from tenant A
	respA := testEnv.DoRequest(t, "GET", "/v1/memories", "", tenantA.APIKey, map[string]string{
		"X-Hermes-User-Id": sharedUserID,
	})
	bodyA := ReadBody(t, respA)

	// Query from tenant B
	respB := testEnv.DoRequest(t, "GET", "/v1/memories", "", tenantB.APIKey, map[string]string{
		"X-Hermes-User-Id": sharedUserID,
	})
	bodyB := ReadBody(t, respB)

	// Tenant A should see "tenant-a-secret", not "tenant-b-secret"
	if containsString(bodyA, "tenant-b-secret") {
		t.Errorf("tenant A sees tenant B's memory: %s", bodyA)
	}

	// Tenant B should see "tenant-b-secret", not "tenant-a-secret"
	if containsString(bodyB, "tenant-a-secret") {
		t.Errorf("tenant B sees tenant A's memory: %s", bodyB)
	}

	// Verify each tenant's own data is present
	var resultA map[string]any
	json.Unmarshal([]byte(bodyA), &resultA)
	if !containsString(bodyA, "tenant-a-secret") {
		t.Errorf("tenant A should see own memory, body: %s", bodyA)
	}
	if !containsString(bodyB, "tenant-b-secret") {
		t.Errorf("tenant B should see own memory, body: %s", bodyB)
	}
}
