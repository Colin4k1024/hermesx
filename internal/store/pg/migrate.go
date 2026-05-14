package pg

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{1, `CREATE TABLE IF NOT EXISTS tenants (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name TEXT NOT NULL,
		plan TEXT NOT NULL DEFAULT 'free',
		rate_limit_rpm INT NOT NULL DEFAULT 60,
		max_sessions INT NOT NULL DEFAULT 100,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},

	{2, `CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		platform TEXT NOT NULL,
		user_id TEXT NOT NULL,
		model TEXT,
		system_prompt TEXT,
		parent_session_id TEXT,
		title TEXT,
		started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		ended_at TIMESTAMPTZ,
		end_reason TEXT,
		message_count INT DEFAULT 0,
		tool_call_count INT DEFAULT 0,
		input_tokens INT DEFAULT 0,
		output_tokens INT DEFAULT 0,
		cache_read_tokens INT DEFAULT 0,
		cache_write_tokens INT DEFAULT 0,
		estimated_cost_usd NUMERIC(10,6),
		metadata JSONB DEFAULT '{}'
	)`},
	{3, `CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id)`},
	{4, `CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(tenant_id, user_id)`},
	{5, `CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(tenant_id, platform)`},

	{6, `CREATE TABLE IF NOT EXISTS messages (
		id BIGSERIAL PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		tool_call_id TEXT,
		tool_calls JSONB,
		tool_name TEXT,
		reasoning TEXT,
		timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
		token_count INT,
		finish_reason TEXT
	)`},
	{7, `CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(tenant_id, session_id)`},
	{8, `CREATE INDEX IF NOT EXISTS idx_messages_ts ON messages(tenant_id, session_id, timestamp)`},
	{9, `CREATE INDEX IF NOT EXISTS idx_messages_fts ON messages USING GIN(to_tsvector('english', coalesce(content, '')))`},

	{10, `CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		external_id TEXT NOT NULL,
		username TEXT,
		display_name TEXT,
		role TEXT DEFAULT 'user',
		approved_at TIMESTAMPTZ,
		metadata JSONB DEFAULT '{}'
	)`},
	{11, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external ON users(tenant_id, external_id)`},

	{12, `CREATE TABLE IF NOT EXISTS audit_logs (
		id BIGSERIAL PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		user_id UUID,
		session_id TEXT,
		action TEXT NOT NULL,
		detail TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{13, `CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_logs(tenant_id)`},

	{14, `CREATE TABLE IF NOT EXISTS cron_jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		prompt TEXT NOT NULL,
		schedule TEXT NOT NULL,
		deliver TEXT,
		enabled BOOLEAN DEFAULT true,
		model TEXT,
		next_run_at TIMESTAMPTZ,
		last_run_at TIMESTAMPTZ,
		run_count INT DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		metadata JSONB DEFAULT '{}'
	)`},
	{15, `CREATE INDEX IF NOT EXISTS idx_cron_tenant ON cron_jobs(tenant_id)`},
	{16, `CREATE INDEX IF NOT EXISTS idx_cron_next ON cron_jobs(next_run_at) WHERE enabled = true`},

	// P1: API keys table
	{17, `CREATE TABLE IF NOT EXISTS api_keys (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		key_hash TEXT NOT NULL,
		prefix TEXT NOT NULL,
		roles TEXT[] DEFAULT '{user}',
		expires_at TIMESTAMPTZ,
		revoked_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{18, `CREATE UNIQUE INDEX IF NOT EXISTS idx_apikeys_hash ON api_keys(key_hash)`},
	{19, `CREATE INDEX IF NOT EXISTS idx_apikeys_tenant ON api_keys(tenant_id)`},

	// P2: Gateway session key + memory tables (v20-v23)
	{20, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS session_key TEXT`},
	{21, `CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_key ON sessions(session_key) WHERE session_key IS NOT NULL`},

	{22, `CREATE TABLE IF NOT EXISTS memories (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		user_id TEXT NOT NULL,
		key TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(tenant_id, user_id, key)
	)`},
	{23, `CREATE TABLE IF NOT EXISTS user_profiles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		user_id TEXT NOT NULL,
		content TEXT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(tenant_id, user_id)
	)`},

	// P3: Audit trail enrichment
	{24, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS request_id TEXT`},
	{25, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS status_code INT`},
	{26, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS latency_ms INT`},
	{27, `CREATE INDEX IF NOT EXISTS idx_audit_request ON audit_logs(request_id)`},

	// P4: Soft delete + cascade + GDPR hardening
	{28, `ALTER TABLE tenants ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`},
	{29, `CREATE INDEX IF NOT EXISTS idx_tenants_active ON tenants(id) WHERE deleted_at IS NULL`},
	{30, `CREATE INDEX IF NOT EXISTS idx_tenants_deleted ON tenants(deleted_at) WHERE deleted_at IS NOT NULL`},

	// Make audit_logs.tenant_id nullable (auth failure events have no tenant context).
	{31, `ALTER TABLE audit_logs ALTER COLUMN tenant_id DROP NOT NULL`},

	// Add source_ip, error_code, user_agent columns to audit_logs.
	{32, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS source_ip TEXT`},
	{33, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS error_code TEXT`},
	{34, `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_agent TEXT`},
	{35, `CREATE INDEX IF NOT EXISTS idx_audit_error_code ON audit_logs(error_code) WHERE error_code IS NOT NULL`},

	// P1-S1: RBAC fine-grained permissions
	{36, `CREATE TABLE IF NOT EXISTS roles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		description TEXT,
		is_system BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(tenant_id, name)
	)`},
	{37, `CREATE TABLE IF NOT EXISTS role_permissions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
		resource TEXT NOT NULL,
		action TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(role_id, resource, action)
	)`},

	// P3-S1: Row-Level Security policies on all tenant-scoped tables.
	{38, `ALTER TABLE sessions ENABLE ROW LEVEL SECURITY`},
	{39, `CREATE POLICY tenant_isolation_sessions ON sessions
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{40, `ALTER TABLE messages ENABLE ROW LEVEL SECURITY`},
	{41, `CREATE POLICY tenant_isolation_messages ON messages
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{42, `ALTER TABLE users ENABLE ROW LEVEL SECURITY`},
	{43, `CREATE POLICY tenant_isolation_users ON users
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{44, `ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY`},
	{45, `CREATE POLICY tenant_isolation_audit ON audit_logs
		USING (tenant_id::text = current_setting('app.current_tenant', true)
		       OR tenant_id IS NULL)`},
	{46, `ALTER TABLE memories ENABLE ROW LEVEL SECURITY`},
	{47, `CREATE POLICY tenant_isolation_memories ON memories
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{48, `ALTER TABLE user_profiles ENABLE ROW LEVEL SECURITY`},
	{49, `CREATE POLICY tenant_isolation_profiles ON user_profiles
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{50, `ALTER TABLE cron_jobs ENABLE ROW LEVEL SECURITY`},
	{51, `CREATE POLICY tenant_isolation_cron ON cron_jobs
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{52, `ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY`},
	{53, `CREATE POLICY tenant_isolation_apikeys ON api_keys
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},
	{54, `ALTER TABLE roles ENABLE ROW LEVEL SECURITY`},
	{55, `CREATE POLICY tenant_isolation_roles ON roles
		USING (tenant_id::text = current_setting('app.current_tenant', true))`},

	// P3-S2: Independent purge audit log table (not subject to RLS).
	{56, `CREATE TABLE IF NOT EXISTS purge_audit_logs (
		id BIGSERIAL PRIMARY KEY,
		tenant_id UUID NOT NULL,
		action TEXT NOT NULL,
		detail TEXT,
		rows_deleted BIGINT DEFAULT 0,
		minio_objects_deleted INT DEFAULT 0,
		duration_ms INT DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},

	// P4-S1: Schema governance — UNIQUE + CHECK constraints.
	{57, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_schema_version_version') THEN
			ALTER TABLE schema_version ADD CONSTRAINT uk_schema_version_version UNIQUE (version);
		END IF;
	END $$`},
	{58, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_tenant_plan') THEN
			ALTER TABLE tenants ADD CONSTRAINT ck_tenant_plan CHECK (plan IN ('free','pro','enterprise'));
		END IF;
	END $$`},
	{59, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_user_role') THEN
			ALTER TABLE users ADD CONSTRAINT ck_user_role CHECK (role IN ('user','admin','operator','viewer','billing'));
		END IF;
	END $$`},

	{60, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'tenants' AND column_name = 'sandbox_policy') THEN
			ALTER TABLE tenants ADD COLUMN sandbox_policy JSONB DEFAULT NULL;
		END IF;
	END $$`},

	// API key scopes for fine-grained permission control.
	{61, `ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scopes TEXT[] DEFAULT '{}'`},

	// P5: Usage metering table for async token recording.
	{62, `CREATE TABLE IF NOT EXISTS usage_records (
		id BIGSERIAL PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL,
		provider TEXT NOT NULL,
		input_tokens INT NOT NULL DEFAULT 0,
		output_tokens INT NOT NULL DEFAULT 0,
		cache_read_tokens INT NOT NULL DEFAULT 0,
		cache_write_tokens INT NOT NULL DEFAULT 0,
		cost_usd NUMERIC(10,6) DEFAULT 0,
		degraded BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`},
	{63, `CREATE INDEX IF NOT EXISTS idx_usage_records_tenant_date ON usage_records(tenant_id, created_at)`},
	{64, `CREATE INDEX IF NOT EXISTS idx_usage_records_session ON usage_records(tenant_id, session_id)`},

	// v1.2.0 P1-S1: FORCE ROW LEVEL SECURITY on all tenant-scoped tables.
	{65, `DO $$ BEGIN
		ALTER TABLE sessions FORCE ROW LEVEL SECURITY;
		ALTER TABLE messages FORCE ROW LEVEL SECURITY;
		ALTER TABLE users FORCE ROW LEVEL SECURITY;
		ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
		ALTER TABLE memories FORCE ROW LEVEL SECURITY;
		ALTER TABLE user_profiles FORCE ROW LEVEL SECURITY;
		ALTER TABLE cron_jobs FORCE ROW LEVEL SECURITY;
		ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
		ALTER TABLE roles FORCE ROW LEVEL SECURITY;
	END $$`},

	// v1.2.0 P1-S1: WITH CHECK policies for INSERT/UPDATE/DELETE on all tenant tables.
	{66, `DO $$ BEGIN
		-- sessions
		DROP POLICY IF EXISTS tenant_write_sessions ON sessions;
		CREATE POLICY tenant_write_sessions ON sessions
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_sessions ON sessions;
		CREATE POLICY tenant_update_sessions ON sessions
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_sessions ON sessions;
		CREATE POLICY tenant_delete_sessions ON sessions
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- messages
		DROP POLICY IF EXISTS tenant_write_messages ON messages;
		CREATE POLICY tenant_write_messages ON messages
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_messages ON messages;
		CREATE POLICY tenant_update_messages ON messages
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_messages ON messages;
		CREATE POLICY tenant_delete_messages ON messages
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- users
		DROP POLICY IF EXISTS tenant_write_users ON users;
		CREATE POLICY tenant_write_users ON users
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_users ON users;
		CREATE POLICY tenant_update_users ON users
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_users ON users;
		CREATE POLICY tenant_delete_users ON users
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- memories
		DROP POLICY IF EXISTS tenant_write_memories ON memories;
		CREATE POLICY tenant_write_memories ON memories
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_memories ON memories;
		CREATE POLICY tenant_update_memories ON memories
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_memories ON memories;
		CREATE POLICY tenant_delete_memories ON memories
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- user_profiles
		DROP POLICY IF EXISTS tenant_write_profiles ON user_profiles;
		CREATE POLICY tenant_write_profiles ON user_profiles
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_profiles ON user_profiles;
		CREATE POLICY tenant_update_profiles ON user_profiles
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_profiles ON user_profiles;
		CREATE POLICY tenant_delete_profiles ON user_profiles
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- cron_jobs
		DROP POLICY IF EXISTS tenant_write_cron ON cron_jobs;
		CREATE POLICY tenant_write_cron ON cron_jobs
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_cron ON cron_jobs;
		CREATE POLICY tenant_update_cron ON cron_jobs
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_cron ON cron_jobs;
		CREATE POLICY tenant_delete_cron ON cron_jobs
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- api_keys
		DROP POLICY IF EXISTS tenant_write_apikeys ON api_keys;
		CREATE POLICY tenant_write_apikeys ON api_keys
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_apikeys ON api_keys;
		CREATE POLICY tenant_update_apikeys ON api_keys
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_apikeys ON api_keys;
		CREATE POLICY tenant_delete_apikeys ON api_keys
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- roles
		DROP POLICY IF EXISTS tenant_write_roles ON roles;
		CREATE POLICY tenant_write_roles ON roles
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_roles ON roles;
		CREATE POLICY tenant_update_roles ON roles
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_roles ON roles;
		CREATE POLICY tenant_delete_roles ON roles
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));

		-- audit_logs: SELECT keeps OR tenant_id IS NULL; write policies use strict check
		DROP POLICY IF EXISTS tenant_isolation_audit ON audit_logs;
		CREATE POLICY tenant_read_audit ON audit_logs
			FOR SELECT USING (tenant_id::text = current_setting('app.current_tenant', true)
			                   OR tenant_id IS NULL);
		DROP POLICY IF EXISTS tenant_write_audit ON audit_logs;
		CREATE POLICY tenant_write_audit ON audit_logs
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_audit ON audit_logs;
		CREATE POLICY tenant_update_audit ON audit_logs
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
	END $$`},

	// v1.2.0 P1-S2: Audit log immutability — revoke DELETE from application role.
	{67, `DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hermes_app') THEN
			REVOKE DELETE ON audit_logs FROM hermes_app;
			REVOKE DELETE, UPDATE ON purge_audit_logs FROM hermes_app;
		END IF;
	END $$`},

	// v1.2.0 P1-S2: GDPR purge function with SECURITY DEFINER (privileged delete).
	{68, `CREATE OR REPLACE FUNCTION gdpr_purge_audit_logs(p_tenant_id TEXT, p_reason TEXT DEFAULT 'GDPR_DELETE')
	RETURNS BIGINT
	LANGUAGE plpgsql
	SECURITY DEFINER
	SET search_path = pg_catalog, public
	AS $$
	DECLARE
		deleted_count BIGINT;
	BEGIN
		SELECT COUNT(*) INTO deleted_count FROM audit_logs WHERE tenant_id = p_tenant_id;
		INSERT INTO purge_audit_logs (tenant_id, action, detail, rows_deleted, created_at)
		VALUES (p_tenant_id, 'GDPR_PURGE_AUDIT', p_reason, deleted_count, now());
		DELETE FROM audit_logs WHERE tenant_id = p_tenant_id;
		RETURN deleted_count;
	END;
	$$`},

	// v1.2.0 P1-S2: pricing_rules table for dynamic pricing (Phase 2 prep).
	{69, `CREATE TABLE IF NOT EXISTS pricing_rules (
		model_key TEXT PRIMARY KEY,
		input_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0,
		output_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0,
		cache_read_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`},

	// v1.2.0 P1-S6: API key expiration support.
	{70, `ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ`},

	// v1.4.0: pg_trgm extension + GIN trigram indexes for CJK full-text search.
	{71, `CREATE EXTENSION IF NOT EXISTS pg_trgm`},
	{72, `CREATE INDEX IF NOT EXISTS idx_messages_trgm ON messages USING GIN(content gin_trgm_ops)`},
	{73, `CREATE INDEX IF NOT EXISTS idx_memories_trgm ON memories USING GIN(content gin_trgm_ops)`},
	{74, `CREATE INDEX IF NOT EXISTS idx_sessions_title_trgm ON sessions USING GIN(title gin_trgm_ops)`},

	// v1.3.0: Execution receipts — auditable tool call records with idempotency.
	{75, `CREATE TABLE IF NOT EXISTS execution_receipts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL DEFAULT '',
		tool_name TEXT NOT NULL,
		input TEXT NOT NULL DEFAULT '',
		output TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'success',
		duration_ms INT NOT NULL DEFAULT 0,
		idempotency_id TEXT,
		trace_id TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{76, `CREATE INDEX IF NOT EXISTS idx_exec_receipts_tenant ON execution_receipts(tenant_id)`},
	{77, `CREATE INDEX IF NOT EXISTS idx_exec_receipts_session ON execution_receipts(tenant_id, session_id)`},
	{78, `CREATE UNIQUE INDEX IF NOT EXISTS idx_exec_receipts_idempotency ON execution_receipts(tenant_id, idempotency_id) WHERE idempotency_id IS NOT NULL AND idempotency_id != ''`},

	// v1.3.0: RLS policy for execution_receipts.
	{79, `ALTER TABLE execution_receipts ENABLE ROW LEVEL SECURITY`},
	{80, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'execution_receipts' AND policyname = 'tenant_isolation_exec_receipts') THEN
			CREATE POLICY tenant_isolation_exec_receipts ON execution_receipts
			USING (tenant_id::text = current_setting('app.current_tenant', true))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
	END $$`},

	// v2.2.0: one-time platform bootstrap claim shared by all API replicas.
	{81, `CREATE TABLE IF NOT EXISTS bootstrap_state (
		id TEXT PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		key_id UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
}

const migrationLockID int64 = 0x48455231 // "HER1" — advisory lock for migration exclusion

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, migrationLockID).Scan(&locked); err != nil {
		return fmt.Errorf("advisory lock check: %w", err)
	}
	if !locked {
		slog.Info("PG migrations skipped — another instance holds the lock")
		return nil
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockID) //nolint:errcheck

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_version (
		version INT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	var current int
	err = conn.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	applied := 0
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := conn.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration v%d failed: %w", m.version, err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO schema_version (version) VALUES ($1)`, m.version); err != nil {
			return fmt.Errorf("record migration v%d: %w", m.version, err)
		}
		applied++
	}

	slog.Info("PG migrations completed", "current", current, "applied", applied, "latest", len(migrations))
	return nil
}
