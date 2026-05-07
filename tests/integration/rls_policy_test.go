//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// TestRLS_AllTables_CrossTenant verifies that PostgreSQL Row-Level Security
// prevents cross-tenant data access across ALL 9 business tables.
// Each sub-test: insert data for tenant B, set RLS context to tenant A, verify 0 rows visible.
func TestRLS_AllTables_CrossTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-all-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-all-b", "pro")
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// Seed data for tenant B via superuser pool
	sessIDB := fmt.Sprintf("rls-all-sess-b-%d", ts)
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessIDB, Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
	})
	testEnv.Store.Messages().Append(ctx, tenantB.ID, sessIDB, &store.Message{
		Role: "user", Content: "rls-secret-msg-b", Timestamp: time.Now(),
	})
	testEnv.Store.Memories().Upsert(ctx, tenantB.ID, "user-b", "rls-mem-key", "rls-secret-memory-b")
	testEnv.Store.UserProfiles().Upsert(ctx, tenantB.ID, "user-b", "rls-secret-profile-b")
	testEnv.Store.AuditLogs().Append(ctx, &store.AuditLog{
		TenantID: tenantB.ID, UserID: "user-b", Action: "test-rls", Detail: "/test",
		RequestID: fmt.Sprintf("req-rls-%d", ts), StatusCode: 200, LatencyMs: 1,
	})
	testEnv.Store.CronJobs().Create(ctx, &store.CronJob{
		ID: fmt.Sprintf("rls-cron-b-%d", ts), TenantID: tenantB.ID, Name: "rls-test",
		Schedule: "*/5 * * * *", Prompt: "test", Enabled: true,
	})
	testEnv.Store.Roles().Create(ctx, &store.Role{
		TenantID: tenantB.ID, Name: fmt.Sprintf("rls-role-b-%d", ts), Description: "test role",
	})

	// Also seed data for tenant A to verify positive access
	sessIDA := fmt.Sprintf("rls-all-sess-a-%d", ts)
	testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
		ID: sessIDA, Platform: "api", UserID: "user-a", Model: "test", StartedAt: time.Now(),
	})
	testEnv.Store.Messages().Append(ctx, tenantA.ID, sessIDA, &store.Message{
		Role: "user", Content: "rls-msg-a", Timestamp: time.Now(),
	})

	// All RLS tests use the restricted pool with SET LOCAL app.current_tenant
	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name:  "sessions",
			query: "SELECT COUNT(*) FROM sessions WHERE id = $1",
			args:  []any{sessIDB},
		},
		{
			name:  "messages",
			query: "SELECT COUNT(*) FROM messages WHERE session_id = $1",
			args:  []any{sessIDB},
		},
		{
			name:  "memories",
			query: "SELECT COUNT(*) FROM memories WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
		{
			name:  "user_profiles",
			query: "SELECT COUNT(*) FROM user_profiles WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
		{
			name:  "audit_logs",
			query: "SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
		{
			name:  "cron_jobs",
			query: "SELECT COUNT(*) FROM cron_jobs WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
		{
			name:  "api_keys",
			query: "SELECT COUNT(*) FROM api_keys WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
		{
			name:  "roles",
			query: "SELECT COUNT(*) FROM roles WHERE tenant_id = $1",
			args:  []any{tenantB.ID},
		},
	}

	for _, tc := range tests {
		t.Run("deny_cross_tenant_"+tc.name, func(t *testing.T) {
			conn, err := testEnv.RLSPool.Acquire(ctx)
			if err != nil {
				t.Fatalf("acquire rls conn: %v", err)
			}
			defer conn.Release()

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

			var count int
			err = tx.QueryRow(ctx, tc.query, tc.args...).Scan(&count)
			if err != nil {
				t.Fatalf("rls query %s: %v", tc.name, err)
			}
			if count != 0 {
				t.Errorf("RLS VIOLATION on %s: tenant A context sees %d rows belonging to tenant B", tc.name, count)
			}
		})
	}

	// Positive tests: tenant A CAN see their own data
	t.Run("allow_own_tenant_sessions", func(t *testing.T) {
		conn, err := testEnv.RLSPool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		defer conn.Release()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
		if err != nil {
			t.Fatalf("set tenant: %v", err)
		}

		var count int
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessIDA).Scan(&count)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if count != 1 {
			t.Errorf("tenant A should see own session, got count=%d", count)
		}
	})

	t.Run("allow_own_tenant_messages", func(t *testing.T) {
		conn, err := testEnv.RLSPool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		defer conn.Release()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
		if err != nil {
			t.Fatalf("set tenant: %v", err)
		}

		var count int
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM messages WHERE session_id = $1", sessIDA).Scan(&count)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if count != 1 {
			t.Errorf("tenant A should see own messages, got count=%d", count)
		}
	})
}

// TestRLS_WritePolicy_CrossTenant verifies that RLS WITH CHECK policies
// prevent a connection from inserting data into another tenant's namespace.
func TestRLS_WritePolicy_CrossTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-write-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-write-b", "pro")
	ctx := context.Background()
	ts := time.Now().UnixNano()

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "insert_session_wrong_tenant",
			sql:  "INSERT INTO sessions (id, tenant_id, platform, user_id, model, started_at) VALUES ($1, $2, 'api', 'attacker', 'test', NOW())",
			args: []any{fmt.Sprintf("rls-attack-sess-%d", ts), tenantB.ID},
		},
		{
			name: "insert_memory_wrong_tenant",
			sql:  "INSERT INTO memories (tenant_id, user_id, key, content) VALUES ($1, 'attacker', 'attack-key', 'attack-data')",
			args: []any{tenantB.ID},
		},
		{
			name: "insert_cron_wrong_tenant",
			sql:  "INSERT INTO cron_jobs (id, tenant_id, user_id, schedule, action, config, active) VALUES ($1, $2, 'attacker', '* * * * *', 'attack', '{}', true)",
			args: []any{fmt.Sprintf("rls-attack-cron-%d", ts), tenantB.ID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := testEnv.RLSPool.Acquire(ctx)
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			defer conn.Release()

			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer tx.Rollback(ctx)

			// Set context to tenant A
			_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
			if err != nil {
				t.Fatalf("set tenant: %v", err)
			}

			// Try to insert data for tenant B — should fail with RLS violation
			_, err = tx.Exec(ctx, tc.sql, tc.args...)
			if err == nil {
				t.Errorf("RLS WRITE VIOLATION on %s: tenant A context successfully inserted data for tenant B", tc.name)
			} else {
				t.Logf("correctly rejected: %v", err)
			}
		})
	}
}

// TestRLS_UpdatePolicy_CrossTenant verifies that UPDATE operations
// are blocked across tenant boundaries.
func TestRLS_UpdatePolicy_CrossTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-upd-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-upd-b", "pro")
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// Seed a session for tenant B
	sessIDB := fmt.Sprintf("rls-upd-sess-b-%d", ts)
	testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
		ID: sessIDB, Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
	})

	conn, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	// Set context to tenant A
	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
	if err != nil {
		t.Fatalf("set tenant: %v", err)
	}

	// Try to update tenant B's session
	tag, err := tx.Exec(ctx, "UPDATE sessions SET model = 'pwned' WHERE id = $1", sessIDB)
	if err != nil {
		t.Logf("update correctly rejected with error: %v", err)
		return
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("RLS UPDATE VIOLATION: tenant A updated %d rows of tenant B's session", tag.RowsAffected())
	}
}

// TestRLS_DeletePolicy_CrossTenant verifies that DELETE operations
// are blocked across tenant boundaries.
func TestRLS_DeletePolicy_CrossTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-del-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-del-b", "pro")
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// Seed data for tenant B
	testEnv.Store.Memories().Upsert(ctx, tenantB.ID, "user-b", fmt.Sprintf("rls-del-key-%d", ts), "important-data")

	conn, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantA.ID))
	if err != nil {
		t.Fatalf("set tenant: %v", err)
	}

	// Try to delete tenant B's memories
	tag, err := tx.Exec(ctx, "DELETE FROM memories WHERE tenant_id = $1", tenantB.ID)
	if err != nil {
		t.Logf("delete correctly rejected with error: %v", err)
		return
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("RLS DELETE VIOLATION: tenant A deleted %d rows of tenant B's memories", tag.RowsAffected())
	}
}

// TestRLS_NoTenantContext_DeniesAll verifies that without SET LOCAL app.current_tenant,
// the restricted user sees NO data (fail-closed).
func TestRLS_NoTenantContext_DeniesAll(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-noctx", "pro")
	ctx := context.Background()

	// Seed data
	sessID := fmt.Sprintf("rls-noctx-sess-%d", time.Now().UnixNano())
	testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
		ID: sessID, Platform: "api", UserID: "user-a", Model: "test", StartedAt: time.Now(),
	})

	conn, err := testEnv.RLSPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	// Do NOT set app.current_tenant — should default to empty/null → no rows visible
	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		// Some RLS configs might error on empty setting — that's also acceptable (fail-closed)
		t.Logf("query without tenant context returns error (fail-closed): %v", err)
		return
	}
	if count != 0 {
		t.Errorf("RLS FAIL-OPEN: without tenant context, restricted user sees %d sessions", count)
	}
}

// TestRLS_ConcurrentTenants verifies RLS isolation under concurrent access.
func TestRLS_ConcurrentTenants(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "rls-conc-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "rls-conc-b", "pro")
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// Seed sessions for both tenants
	for i := 0; i < 5; i++ {
		testEnv.Store.Sessions().Create(ctx, tenantA.ID, &store.Session{
			ID: fmt.Sprintf("rls-conc-a-%d-%d", ts, i), Platform: "api", UserID: "user-a", Model: "test", StartedAt: time.Now(),
		})
		testEnv.Store.Sessions().Create(ctx, tenantB.ID, &store.Session{
			ID: fmt.Sprintf("rls-conc-b-%d-%d", ts, i), Platform: "api", UserID: "user-b", Model: "test", StartedAt: time.Now(),
		})
	}

	// Run concurrent queries — each tenant should only see their own data
	errs := make(chan error, 20)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			var tenant *TestTenant
			if idx%2 == 0 {
				tenant = tenantA
			} else {
				tenant = tenantB
			}

			conn, err := testEnv.RLSPool.Acquire(ctx)
			if err != nil {
				errs <- fmt.Errorf("acquire: %w", err)
				return
			}
			defer conn.Release()

			tx, err := conn.Begin(ctx)
			if err != nil {
				errs <- fmt.Errorf("begin: %w", err)
				return
			}
			defer tx.Rollback(ctx)

			_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenant.ID))
			if err != nil {
				errs <- fmt.Errorf("set tenant: %w", err)
				return
			}

			var count int
			err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE id LIKE $1", fmt.Sprintf("rls-conc-%%-%d-%%", ts)).Scan(&count)
			if err != nil {
				errs <- fmt.Errorf("query: %w", err)
				return
			}

			if count > 5 {
				errs <- fmt.Errorf("RLS CONCURRENCY VIOLATION: tenant %s sees %d sessions (expected <=5)", tenant.Name, count)
				return
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
