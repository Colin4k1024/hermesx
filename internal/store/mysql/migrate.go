package mysql

import (
	"context"
	"database/sql"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS tenants (
		id           CHAR(36)     NOT NULL PRIMARY KEY,
		name         VARCHAR(255) NOT NULL,
		plan         VARCHAR(50)  NOT NULL DEFAULT 'free',
		rate_limit_rpm INT        NOT NULL DEFAULT 60,
		max_sessions  INT         NOT NULL DEFAULT 5,
		sandbox_policy TEXT       NULL,
		created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		updated_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		deleted_at   DATETIME(3)  NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS sessions (
		id                CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id         CHAR(36)     NOT NULL,
		platform          VARCHAR(50)  NOT NULL DEFAULT '',
		user_id           VARCHAR(255) NOT NULL DEFAULT '',
		model             VARCHAR(100) NOT NULL DEFAULT '',
		system_prompt     TEXT         NULL,
		parent_session_id CHAR(36)     NULL,
		title             VARCHAR(500) NULL,
		started_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		ended_at          DATETIME(3)  NULL,
		end_reason        VARCHAR(100) NULL,
		message_count     INT          NOT NULL DEFAULT 0,
		tool_call_count   INT          NOT NULL DEFAULT 0,
		input_tokens      INT          NOT NULL DEFAULT 0,
		output_tokens     INT          NOT NULL DEFAULT 0,
		cache_read_tokens INT          NOT NULL DEFAULT 0,
		cache_write_tokens INT         NOT NULL DEFAULT 0,
		estimated_cost_usd DOUBLE      NOT NULL DEFAULT 0,
		INDEX idx_sessions_tenant (tenant_id),
		INDEX idx_sessions_started (tenant_id, started_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS messages (
		id            BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
		tenant_id     CHAR(36)     NOT NULL,
		session_id    CHAR(36)     NOT NULL,
		role          VARCHAR(50)  NOT NULL,
		content       MEDIUMTEXT   NULL,
		tool_call_id  VARCHAR(255) NULL,
		tool_calls    MEDIUMTEXT   NULL,
		tool_name     VARCHAR(255) NULL,
		reasoning     MEDIUMTEXT   NULL,
		timestamp     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		token_count   INT          NULL,
		finish_reason VARCHAR(100) NULL,
		INDEX idx_messages_session (tenant_id, session_id),
		INDEX idx_messages_ts (tenant_id, session_id, timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS users (
		id           CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id    CHAR(36)     NOT NULL,
		external_id  VARCHAR(500) NOT NULL,
		username     VARCHAR(255) NULL,
		display_name VARCHAR(255) NULL,
		role         VARCHAR(50)  NOT NULL DEFAULT 'user',
		approved_at  DATETIME(3)  NULL,
		UNIQUE KEY uk_users_ext (tenant_id, external_id),
		INDEX idx_users_tenant (tenant_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS audit_logs (
		id           BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
		tenant_id    CHAR(36)     NULL,
		user_id      CHAR(36)     NULL,
		session_id   VARCHAR(255) NULL,
		action       VARCHAR(255) NOT NULL,
		detail       TEXT         NULL,
		request_id   VARCHAR(255) NULL,
		status_code  INT          NULL,
		latency_ms   INT          NULL,
		source_ip    VARCHAR(100) NULL,
		error_code   VARCHAR(100) NULL,
		user_agent   VARCHAR(500) NULL,
		created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		INDEX idx_audit_tenant (tenant_id, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS api_keys (
		id         CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id  CHAR(36)     NOT NULL,
		name       VARCHAR(255) NOT NULL,
		key_hash   CHAR(64)     NOT NULL,
		prefix     VARCHAR(20)  NOT NULL,
		roles      VARCHAR(1000) NOT NULL DEFAULT '[]',
		scopes     VARCHAR(1000) NOT NULL DEFAULT '[]',
		expires_at DATETIME(3)  NULL,
		revoked_at DATETIME(3)  NULL,
		created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_key_hash (key_hash),
		INDEX idx_apikey_tenant (tenant_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS memories (
		tenant_id  CHAR(36)     NOT NULL,
		user_id    VARCHAR(255) NOT NULL,
		key_name   VARCHAR(255) NOT NULL,
		content    MEDIUMTEXT   NOT NULL,
		updated_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		PRIMARY KEY (tenant_id, user_id, key_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS user_profiles (
		tenant_id  CHAR(36)     NOT NULL,
		user_id    VARCHAR(255) NOT NULL,
		content    MEDIUMTEXT   NOT NULL,
		updated_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		PRIMARY KEY (tenant_id, user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS cron_jobs (
		id         CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id  CHAR(36)     NOT NULL,
		name       VARCHAR(255) NOT NULL,
		prompt     TEXT         NOT NULL,
		schedule   VARCHAR(100) NOT NULL,
		deliver    VARCHAR(255) NULL,
		enabled    TINYINT(1)   NOT NULL DEFAULT 1,
		model      VARCHAR(100) NULL,
		next_run_at DATETIME(3) NULL,
		last_run_at DATETIME(3) NULL,
		run_count  INT          NOT NULL DEFAULT 0,
		created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		metadata   TEXT         NULL,
		INDEX idx_cronjob_tenant (tenant_id),
		INDEX idx_cronjob_due (enabled, next_run_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS roles (
		id          CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id   CHAR(36)     NOT NULL,
		name        VARCHAR(100) NOT NULL,
		description TEXT         NULL,
		is_system   TINYINT(1)   NOT NULL DEFAULT 0,
		created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_role_name (tenant_id, name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS role_permissions (
		id         CHAR(36)     NOT NULL PRIMARY KEY,
		role_id    CHAR(36)     NOT NULL,
		resource   VARCHAR(255) NOT NULL,
		action     VARCHAR(100) NOT NULL,
		created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_perm (role_id, resource, action),
		INDEX idx_perm_role (role_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS pricing_rules (
		model_key       VARCHAR(200) NOT NULL PRIMARY KEY,
		input_per_1k    DOUBLE       NOT NULL DEFAULT 0,
		output_per_1k   DOUBLE       NOT NULL DEFAULT 0,
		cache_read_per_1k DOUBLE     NOT NULL DEFAULT 0,
		updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS execution_receipts (
		id             CHAR(36)     NOT NULL PRIMARY KEY,
		tenant_id      CHAR(36)     NOT NULL,
		session_id     CHAR(36)     NOT NULL,
		user_id        VARCHAR(255) NOT NULL,
		tool_name      VARCHAR(255) NOT NULL,
		input          MEDIUMTEXT   NOT NULL,
		output         MEDIUMTEXT   NOT NULL,
		status         VARCHAR(50)  NOT NULL,
		duration_ms    INT          NOT NULL DEFAULT 0,
		idempotency_id VARCHAR(255) NULL,
		trace_id       VARCHAR(255) NULL,
		created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		INDEX idx_receipt_tenant (tenant_id, created_at),
		INDEX idx_receipt_idem (tenant_id, idempotency_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	// Ensure version-tracking table exists.
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT NOT NULL PRIMARY KEY,
		applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	if err != nil {
		return fmt.Errorf("mysql create schema_migrations: %w", err)
	}

	for i, m := range migrations {
		var count int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, i).Scan(&count); err != nil {
			return fmt.Errorf("mysql migration version check %d: %w", i, err)
		}
		if count > 0 {
			continue // already applied
		}
		if _, err := db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("mysql migration %d: %w", i, err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, i); err != nil {
			return fmt.Errorf("mysql migration record %d: %w", i, err)
		}
	}
	return nil
}
