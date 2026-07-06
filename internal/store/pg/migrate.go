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

	// v2.3.0: fixed SOP workflow definitions, immutable versions, and runtime state.
	{82, `CREATE TABLE IF NOT EXISTS workflow_definitions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		graph_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		latest_version_id UUID,
		created_by TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{83, `CREATE TABLE IF NOT EXISTS workflow_versions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		definition_id UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
		version INT NOT NULL,
		graph_json JSONB NOT NULL,
		published_by TEXT NOT NULL DEFAULT '',
		published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (definition_id, version)
	)`},
	{84, `CREATE TABLE IF NOT EXISTS workflow_runs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		definition_id UUID NOT NULL REFERENCES workflow_definitions(id),
		version_id UUID NOT NULL REFERENCES workflow_versions(id),
		status TEXT NOT NULL DEFAULT 'pending',
		started_by TEXT NOT NULL DEFAULT '',
		input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		variables_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		error TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		completed_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{85, `CREATE TABLE IF NOT EXISTS workflow_step_runs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id),
		run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
		node_id TEXT NOT NULL,
		node_type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempt INT NOT NULL DEFAULT 0,
		assignee_user_id TEXT NOT NULL DEFAULT '',
		assignee_role TEXT NOT NULL DEFAULT '',
		input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		output_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		outcome TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ,
		completed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (run_id, node_id)
	)`},
	{86, `CREATE INDEX IF NOT EXISTS idx_workflow_defs_tenant ON workflow_definitions(tenant_id, created_at DESC)`},
	{87, `CREATE INDEX IF NOT EXISTS idx_workflow_versions_def ON workflow_versions(tenant_id, definition_id, version DESC)`},
	{88, `CREATE INDEX IF NOT EXISTS idx_workflow_runs_tenant ON workflow_runs(tenant_id, started_at DESC)`},
	{89, `CREATE INDEX IF NOT EXISTS idx_workflow_steps_run ON workflow_step_runs(tenant_id, run_id)`},
	{90, `CREATE INDEX IF NOT EXISTS idx_workflow_steps_human ON workflow_step_runs(tenant_id, status, assignee_user_id, assignee_role)`},
	{91, `ALTER TABLE workflow_definitions ENABLE ROW LEVEL SECURITY`},
	{92, `ALTER TABLE workflow_versions ENABLE ROW LEVEL SECURITY`},
	{93, `ALTER TABLE workflow_runs ENABLE ROW LEVEL SECURITY`},
	{94, `ALTER TABLE workflow_step_runs ENABLE ROW LEVEL SECURITY`},
	{95, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'workflow_definitions' AND policyname = 'tenant_isolation_workflow_definitions') THEN
			CREATE POLICY tenant_isolation_workflow_definitions ON workflow_definitions
			USING (tenant_id::text = current_setting('app.current_tenant', true))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'workflow_versions' AND policyname = 'tenant_isolation_workflow_versions') THEN
			CREATE POLICY tenant_isolation_workflow_versions ON workflow_versions
			USING (tenant_id::text = current_setting('app.current_tenant', true))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'workflow_runs' AND policyname = 'tenant_isolation_workflow_runs') THEN
			CREATE POLICY tenant_isolation_workflow_runs ON workflow_runs
			USING (tenant_id::text = current_setting('app.current_tenant', true))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'workflow_step_runs' AND policyname = 'tenant_isolation_workflow_step_runs') THEN
			CREATE POLICY tenant_isolation_workflow_step_runs ON workflow_step_runs
			USING (tenant_id::text = current_setting('app.current_tenant', true))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
	END $$`},
	{96, `ALTER TABLE workflow_definitions FORCE ROW LEVEL SECURITY`},
	{97, `ALTER TABLE workflow_versions FORCE ROW LEVEL SECURITY`},
	{98, `ALTER TABLE workflow_runs FORCE ROW LEVEL SECURITY`},
	{99, `ALTER TABLE workflow_step_runs FORCE ROW LEVEL SECURITY`},
	{100, `CREATE TABLE IF NOT EXISTS cron_job_runs (
		id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
		cron_job_id    UUID        NOT NULL REFERENCES cron_jobs(id) ON DELETE CASCADE,
		tenant_id      VARCHAR(64) NOT NULL,
		status         VARCHAR(16) NOT NULL DEFAULT 'pending',
		scheduled_at   TIMESTAMPTZ NOT NULL,
		started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
		finished_at    TIMESTAMPTZ,
		duration_ms    INT,
		result         TEXT,
		error          TEXT,
		pod_id         VARCHAR(128),
		created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
		CONSTRAINT uq_cron_job_runs_job_scheduled UNIQUE (cron_job_id, scheduled_at)
	);
	CREATE INDEX IF NOT EXISTS idx_cron_job_runs_job_id ON cron_job_runs(cron_job_id);
	CREATE INDEX IF NOT EXISTS idx_cron_job_runs_tenant ON cron_job_runs(tenant_id, started_at DESC)`},
	{101, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='cron_jobs' AND column_name='last_run_success') THEN
			ALTER TABLE cron_jobs ADD COLUMN last_run_success boolean;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='cron_jobs' AND column_name='last_run_error') THEN
			ALTER TABLE cron_jobs ADD COLUMN last_run_error text;
		END IF;
	END $$`},
	{102, `ALTER TABLE cron_job_runs ENABLE ROW LEVEL SECURITY;
	ALTER TABLE cron_job_runs FORCE ROW LEVEL SECURITY;
	DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'cron_job_runs' AND policyname = 'tenant_isolation_cron_runs') THEN
			CREATE POLICY tenant_isolation_cron_runs ON cron_job_runs
				USING (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
	END $$`},
	{103, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='cron_jobs' AND column_name='source_platform') THEN
			ALTER TABLE cron_jobs ADD COLUMN source_platform VARCHAR(64);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='cron_jobs' AND column_name='source_chat_id') THEN
			ALTER TABLE cron_jobs ADD COLUMN source_chat_id VARCHAR(255);
		END IF;
	END $$`},

	{104, `CREATE INDEX IF NOT EXISTS idx_cron_job_runs_tenant_job_started
		ON cron_job_runs (tenant_id, cron_job_id, started_at DESC)`},

	// v1.5.1: Write policies for cron_job_runs (required for FORCE RLS + scheduler writes).
	{105, `DO $$ BEGIN
		DROP POLICY IF EXISTS tenant_write_cron_runs ON cron_job_runs;
		CREATE POLICY tenant_write_cron_runs ON cron_job_runs
			FOR INSERT WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_update_cron_runs ON cron_job_runs;
		CREATE POLICY tenant_update_cron_runs ON cron_job_runs
			FOR UPDATE USING (tenant_id::text = current_setting('app.current_tenant', false))
			WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		DROP POLICY IF EXISTS tenant_delete_cron_runs ON cron_job_runs;
		CREATE POLICY tenant_delete_cron_runs ON cron_job_runs
			FOR DELETE USING (tenant_id::text = current_setting('app.current_tenant', false));
	END $$`},

	// v1.5.1: Scheduler cleanup function — cross-tenant stale run recovery (SECURITY DEFINER).
	{106, `CREATE OR REPLACE FUNCTION scheduler_cleanup_stale_runs(p_lock_ttl_seconds INT)
	RETURNS BIGINT
	LANGUAGE plpgsql
	SECURITY DEFINER
	SET search_path = pg_catalog, public
	AS $$
	DECLARE
		cleaned BIGINT;
	BEGIN
		UPDATE cron_job_runs
		SET    status = 'failed',
		       error  = 'pod crash or lock TTL exceeded',
		       finished_at = now()
		WHERE  status = 'running'
		  AND  started_at < now() - (p_lock_ttl_seconds || ' seconds')::interval;
		GET DIAGNOSTICS cleaned = ROW_COUNT;
		RETURN cleaned;
	END;
	$$`},

	{107, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='agentic_blocks') THEN
			ALTER TABLE messages ADD COLUMN agentic_blocks JSONB;
		END IF;
	END $$;
	CREATE TABLE IF NOT EXISTS agent_checkpoints (
		tenant_id     UUID        NOT NULL,
		session_id    VARCHAR(64) NOT NULL,
		checkpoint_id TEXT        NOT NULL,
		payload       BYTEA       NOT NULL,
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (tenant_id, session_id, checkpoint_id)
	);
	CREATE INDEX IF NOT EXISTS idx_agent_checkpoints_tenant_updated
		ON agent_checkpoints (tenant_id, updated_at DESC);
	ALTER TABLE agent_checkpoints ENABLE ROW LEVEL SECURITY;
	ALTER TABLE agent_checkpoints FORCE ROW LEVEL SECURITY;
	DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'agent_checkpoints' AND policyname = 'tenant_isolation_agent_checkpoints') THEN
			CREATE POLICY tenant_isolation_agent_checkpoints ON agent_checkpoints
				USING (tenant_id::text = current_setting('app.current_tenant', true));
		END IF;
	END $$`},

	// v2.4.0-dev: tenant egress allowlist rules consumed by SecureTransport.
	{108, `CREATE TABLE IF NOT EXISTS egress_rules (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		host_pattern TEXT NOT NULL,
		path_prefix TEXT NOT NULL DEFAULT '/',
		action TEXT NOT NULL CHECK (action IN ('allow','deny')),
		priority INT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS idx_egress_rules_tenant_priority
		ON egress_rules (tenant_id, priority DESC);
	CREATE INDEX IF NOT EXISTS idx_egress_rules_tenant_host
		ON egress_rules (tenant_id, host_pattern)`},

	// v2.3.0 P3-S1 fix: execution_receipts missing FORCE RLS (defense-in-depth; #27).
	{109, `ALTER TABLE execution_receipts FORCE ROW LEVEL SECURITY`},

	// v2.3.0 P3-S1 fix: egress_rules RLS — tenant isolation for allowlist table (#27).
	{110, `ALTER TABLE egress_rules ENABLE ROW LEVEL SECURITY;
	ALTER TABLE egress_rules FORCE ROW LEVEL SECURITY;
	DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'egress_rules' AND policyname = 'tenant_isolation_egress_rules') THEN
			CREATE POLICY tenant_isolation_egress_rules ON egress_rules
				USING (tenant_id::text = current_setting('app.current_tenant', true))
				WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		END IF;
	END $$`},

	// v2.4.0-dev: trusted channel login and gateway binding.
	{111, `CREATE TABLE IF NOT EXISTS channel_apps (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		platform TEXT NOT NULL,
		app_key TEXT NOT NULL,
		app_secret_ref TEXT,
		oauth_secret_ref TEXT,
		webhook_secret_ref TEXT,
		enabled BOOLEAN NOT NULL DEFAULT true,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(platform, app_key)
	);
	CREATE INDEX IF NOT EXISTS idx_channel_apps_tenant ON channel_apps(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_channel_apps_lookup ON channel_apps(platform, app_key) WHERE deleted_at IS NULL`},
	{112, `CREATE TABLE IF NOT EXISTS channel_identities (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		channel_app_id UUID NOT NULL REFERENCES channel_apps(id) ON DELETE CASCADE,
		platform TEXT NOT NULL,
		provider_user_hash TEXT NOT NULL,
		provider_display_name TEXT,
		user_id TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		last_login_at TIMESTAMPTZ,
		revoked_at TIMESTAMPTZ,
		UNIQUE(channel_app_id, provider_user_hash)
	);
	CREATE INDEX IF NOT EXISTS idx_channel_identities_tenant_user ON channel_identities(tenant_id, user_id);
	CREATE INDEX IF NOT EXISTS idx_channel_identities_lookup ON channel_identities(channel_app_id, provider_user_hash)`},
	{113, `CREATE TABLE IF NOT EXISTS browser_sessions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		csrf_token_hash TEXT NOT NULL,
		user_agent TEXT,
		source_ip TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		last_seen_at TIMESTAMPTZ,
		expires_at TIMESTAMPTZ NOT NULL,
		revoked_at TIMESTAMPTZ
	);
	CREATE INDEX IF NOT EXISTS idx_browser_sessions_tenant_user ON browser_sessions(tenant_id, user_id);
	CREATE INDEX IF NOT EXISTS idx_browser_sessions_active ON browser_sessions(token_hash) WHERE revoked_at IS NULL`},

	// v2.4.0: Index for audit log archival (efficient range scan by created_at for retention jobs).
	{114, `CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at ASC)`},

	// v2.5.0: Alert rules and events tables for usage alerting.
	{115, `CREATE TABLE IF NOT EXISTS alert_rules (
		id TEXT PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		metric TEXT NOT NULL,
		threshold NUMERIC(14,4) NOT NULL DEFAULT 0,
		alert_window TEXT NOT NULL DEFAULT 'daily',
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{116, `CREATE TABLE IF NOT EXISTS alert_events (
		id TEXT PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		rule_id TEXT NOT NULL,
		metric TEXT NOT NULL,
		threshold NUMERIC(14,4) NOT NULL DEFAULT 0,
		current_val NUMERIC(14,4) NOT NULL DEFAULT 0,
		percentage NUMERIC(10,2) NOT NULL DEFAULT 0,
		fired_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`},
	{117, `CREATE INDEX IF NOT EXISTS idx_alert_rules_tenant ON alert_rules(tenant_id)`},
	{118, `CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled) WHERE enabled = true`},
	{119, `CREATE INDEX IF NOT EXISTS idx_alert_events_tenant ON alert_events(tenant_id, fired_at DESC)`},
	{120, `CREATE INDEX IF NOT EXISTS idx_alert_events_rule ON alert_events(tenant_id, rule_id)`},

	// v2.5.0: RLS policies for alert_rules and alert_events.
	{121, `ALTER TABLE alert_rules ENABLE ROW LEVEL SECURITY;
	ALTER TABLE alert_rules FORCE ROW LEVEL SECURITY;
	DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'alert_rules' AND policyname = 'tenant_isolation_alert_rules') THEN
			CREATE POLICY tenant_isolation_alert_rules ON alert_rules
				USING (tenant_id::text = current_setting('app.current_tenant', true))
				WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		END IF;
	END $$`},
	{122, `ALTER TABLE alert_events ENABLE ROW LEVEL SECURITY;
	ALTER TABLE alert_events FORCE ROW LEVEL SECURITY;
	DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'alert_events' AND policyname = 'tenant_isolation_alert_events') THEN
			CREATE POLICY tenant_isolation_alert_events ON alert_events
				USING (tenant_id::text = current_setting('app.current_tenant', true))
				WITH CHECK (tenant_id::text = current_setting('app.current_tenant', false));
		END IF;
	END $$`},

	// v2.5.1: Per-tenant safety policy store.
	{123, `CREATE TABLE IF NOT EXISTS safety_policies (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		mode TEXT NOT NULL DEFAULT 'log_only',
		input_patterns JSONB NOT NULL DEFAULT '[]'::jsonb,
		output_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (tenant_id),
		CONSTRAINT ck_safety_policy_mode CHECK (mode IN ('enforce', 'log_only', 'disabled'))
	);
	CREATE INDEX IF NOT EXISTS idx_safety_policies_tenant ON safety_policies(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_safety_policies_updated ON safety_policies(updated_at DESC)`},

	// v2.6.0: File workspace — hybrid MinIO + PG model for user workspace files.
	{124, `CREATE TABLE IF NOT EXISTS file_entries (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL,
		path TEXT NOT NULL,
		minio_key TEXT NOT NULL,
		size_bytes BIGINT NOT NULL DEFAULT 0,
		mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		sha256 TEXT NOT NULL DEFAULT '',
		source_session TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		deleted_at TIMESTAMPTZ
	)`},
	{125, `CREATE INDEX IF NOT EXISTS idx_file_entries_user ON file_entries(tenant_id, user_id, deleted_at)`},
	{126, `CREATE UNIQUE INDEX IF NOT EXISTS idx_file_entries_path ON file_entries(tenant_id, user_id, path) WHERE deleted_at IS NULL`},
	{127, `ALTER TABLE file_entries ENABLE ROW LEVEL SECURITY`},
	{128, `CREATE POLICY tenant_isolation_file_entries ON file_entries USING (tenant_id::text = current_setting('app.current_tenant', true)) WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true))`},
	{129, `ALTER TABLE file_entries FORCE ROW LEVEL SECURITY`},
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
