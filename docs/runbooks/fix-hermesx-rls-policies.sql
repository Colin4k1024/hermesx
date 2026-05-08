-- ============================================================
-- HermesX RLS Policy Fix — memories / user_profiles INSERT/UPDATE/DELETE
-- ============================================================
-- Applies to: hermesx database (PostgreSQL 16+)
-- Issue:    ON CONFLICT DO UPDATE in PGMemoryProvider.SaveMemory/SaveUserProfile
--           triggers the UPDATE RLS policy which requires app.current_tenant to be set.
--           Additionally, cmd=ALL isolation policies block SELECT without tenant context.
-- Fix:      All memory CRUD operations now wrap in a transaction with
--           set_config('app.current_tenant', $1, true) before executing the query.
--           The transaction scopes the session variable so it is rolled back
--           when the transaction ends, avoiding connection-level tenant leakage.
--
-- This SQL file documents the policy state. The actual fix is in Go code:
--   internal/agent/memory_pg.go   — SaveMemory, ReadMemory, ReadUserProfile,
--                                   SaveUserProfile, DeleteMemory now use
--                                   tenant-scoped transactions.
--   internal/api/memory_api.go   — handleListMemories uses WithTenantTx.
--   internal/api/memory_extractor.go — persist() uses WithTenantTx.
--   internal/store/pg/tenant_ctx.go — WithTenantTx exported.
-- ============================================================

-- ----------------------------------------------------------------
-- memories table policies (as of v1.4.0)
-- ----------------------------------------------------------------
-- SELECT:  tenant_read_memories
--   qual: tenant_id::text = current_setting('app.current_tenant', true)
--   With the 'true' arg, returns NULL when not set → row invisible (safe default)
--
-- INSERT:  tenant_write_memories
--   qual:  (empty)
--   with_check: tenant_id::text = current_setting('app.current_tenant', false)
--   The with_check is the blocking part — requires tenant context
--
-- UPDATE:  tenant_update_memories
--   qual:      tenant_id::text = current_setting('app.current_tenant', false)
--   with_check: tenant_id::text = current_setting('app.current_tenant', false)
--   Both error if not set ('false' arg raises ERROR on missing setting)
--
-- DELETE:  tenant_delete_memories
--   qual: tenant_id::text = current_setting('app.current_tenant', false)
--   Errors if not set
--
-- ALL (isolation): tenant_isolation_memories
--   qual: tenant_id::text = current_setting('app.current_tenant', true)
--   Applied to ALL commands — blocks SELECT when not set (safe default)

-- ----------------------------------------------------------------
-- user_profiles table policies (same pattern as memories)
-- ----------------------------------------------------------------
-- INSERT: tenant_write_profiles  — with_check: tenant_id = set_config(..., false)
-- UPDATE: tenant_update_profiles — requires tenant context
-- DELETE: tenant_delete_profiles — requires tenant context
-- SELECT (via ALL isolation): requires tenant context

-- ----------------------------------------------------------------
-- tenant context setting pattern
-- ----------------------------------------------------------------
-- Every write/read operation that touches RLS-protected tables
-- must set the tenant context inside a transaction:
--
--   BEGIN;
--   SELECT set_config('app.current_tenant', $1, true);  -- true = session-scoped
--   -- execute query here --
--   COMMIT;  -- rolls back the session variable automatically
--
-- The 'true' argument makes set_config session-scoped (not just transaction-scoped),
-- but wrapping in a transaction means the setting is rolled back when the
-- transaction rolls back, preventing cross-request tenant leakage.
--
-- Using 'false' (transaction-scoped) would also work, but set_config
-- with 'false' errors if already set in a parent context, so 'true' is safer.

-- ----------------------------------------------------------------
-- Verification query
-- ----------------------------------------------------------------
-- Check that all expected policies exist:
SELECT tablename, policyname, cmd, permissive,
       CASE WHEN qual IS NULL OR qual = '' THEN '(none)' ELSE qual END AS qual,
       CASE WHEN with_check IS NULL OR with_check = '' THEN '(none)' ELSE with_check END AS with_check
FROM pg_policies
WHERE tablename IN ('memories', 'user_profiles', 'messages', 'sessions',
                    'api_keys', 'tenants', 'audit_logs', 'pricing_rules',
                    'execution_receipts')
ORDER BY tablename, cmd;
