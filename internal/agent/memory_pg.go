package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ MemoryProvider = (*PGMemoryProvider)(nil)
var _ SystemPromptProvider = (*PGMemoryProvider)(nil)
var _ ShutdownProvider = (*PGMemoryProvider)(nil)

// PGMemoryProvider implements MemoryProvider backed by PostgreSQL.
type PGMemoryProvider struct {
	pool     *pgxpool.Pool
	tenantID string
	userID   string
}

func NewPGMemoryProvider(pool *pgxpool.Pool, tenantID, userID string) *PGMemoryProvider {
	return &PGMemoryProvider{pool: pool, tenantID: tenantID, userID: userID}
}

func (p *PGMemoryProvider) ReadMemory() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const maxEntries = 50
	const maxBytes = 8192

	// Wrap in a transaction so the tenant context is scoped and rolled back,
	// avoiding connection-level leakage across pooled connection reuse.
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("pg begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", p.tenantID); err != nil {
		return "", fmt.Errorf("pg set tenant context: %w", err)
	}

	rows, err := tx.Query(ctx,
		`SELECT key, content FROM memories
		 WHERE tenant_id = $1 AND user_id = $2
		 ORDER BY updated_at DESC
		 LIMIT $3`,
		p.tenantID, p.userID, maxEntries)
	if err != nil {
		return "", fmt.Errorf("pg read memory: %w", err)
	}
	defer rows.Close()

	var parts []string
	totalBytes := 0
	for rows.Next() {
		var key, content string
		if err := rows.Scan(&key, &content); err != nil {
			continue
		}
		entry := fmt.Sprintf("## %s\n%s", key, content)
		if totalBytes+len(entry) > maxBytes {
			break
		}
		parts = append(parts, entry)
		totalBytes += len(entry)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("pg iterate memories: %w", err)
	}

	slog.Debug("pg_read_memory", "tenant", p.tenantID, "user", p.userID, "entries", len(parts), "bytes", totalBytes)
	return strings.Join(parts, "\n\n"), nil
}

func (p *PGMemoryProvider) SaveMemory(key, content string) error {
	if key == "" || content == "" {
		return fmt.Errorf("both key and content are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wrap in a transaction so the set_config session variable is scoped to this
	// operation and rolled back when the transaction ends, avoiding connection-level
	// tenant leakage across pooled connection reuse.
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pg begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", p.tenantID); err != nil {
		return fmt.Errorf("pg set tenant context: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO memories (tenant_id, user_id, key, content, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, user_id, key)
		 DO UPDATE SET content = $4, updated_at = now()`,
		p.tenantID, p.userID, key, content)
	if err != nil {
		return fmt.Errorf("pg save memory: %w", err)
	}

	return tx.Commit(ctx)
}

// SaveMemoryTx saves a memory within a transaction that already has the RLS tenant
// context set (via set_config('app.current_tenant', ..., true)). This avoids the
// ON CONFLICT UPDATE portion of the UPSERT triggering the UPDATE policy when called
// from contexts like memory_extractor that don't set app.current_tenant on the pool.
// The caller provides ctx so it can manage its own timeout/scope.
func (p *PGMemoryProvider) SaveMemoryTx(ctx context.Context, tx pgx.Tx, key, content string) error {
	if key == "" || content == "" {
		return fmt.Errorf("both key and content are required")
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO memories (tenant_id, user_id, key, content, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, user_id, key)
		 DO UPDATE SET content = $4, updated_at = now()`,
		p.tenantID, p.userID, key, content)
	if err != nil {
		return fmt.Errorf("pg save memory tx: %w", err)
	}
	return nil
}

func (p *PGMemoryProvider) DeleteMemory(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pg begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", p.tenantID); err != nil {
		return fmt.Errorf("pg set tenant context: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM memories WHERE tenant_id = $1 AND user_id = $2 AND key = $3`,
		p.tenantID, p.userID, key)
	if err != nil {
		return fmt.Errorf("pg delete memory: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pg commit: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory key '%s' not found", key)
	}
	return nil
}

func (p *PGMemoryProvider) ReadUserProfile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("pg begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", p.tenantID); err != nil {
		return "", fmt.Errorf("pg set tenant context: %w", err)
	}

	var content string
	err = tx.QueryRow(ctx,
		`SELECT content FROM user_profiles
		 WHERE tenant_id = $1 AND user_id = $2`,
		p.tenantID, p.userID).Scan(&content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("pg read user profile: %w", err)
	}
	return content, nil
}

func (p *PGMemoryProvider) SaveUserProfile(content string) error {
	if content == "" {
		return fmt.Errorf("content is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pg begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", p.tenantID); err != nil {
		return fmt.Errorf("pg set tenant context: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO user_profiles (tenant_id, user_id, content, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (tenant_id, user_id)
		 DO UPDATE SET content = $3, updated_at = now()`,
		p.tenantID, p.userID, content)
	if err != nil {
		return fmt.Errorf("pg save user profile: %w", err)
	}

	return tx.Commit(ctx)
}

// SaveUserProfileTx saves a user profile within a transaction with RLS tenant context set.
func (p *PGMemoryProvider) SaveUserProfileTx(ctx context.Context, tx pgx.Tx, content string) error {
	if content == "" {
		return fmt.Errorf("content is required")
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO user_profiles (tenant_id, user_id, content, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (tenant_id, user_id)
		 DO UPDATE SET content = $3, updated_at = now()`,
		p.tenantID, p.userID, content)
	if err != nil {
		return fmt.Errorf("pg save user profile tx: %w", err)
	}
	return nil
}

func (p *PGMemoryProvider) SystemPromptBlock() string {
	var parts []string

	profile, err := p.ReadUserProfile()
	if err != nil {
		slog.Warn("PG: failed to read user profile for system prompt", "error", err)
	} else if profile != "" {
		parts = append(parts, "## Known User Profile\nThe following is what you already know about this user. Use it to personalize responses.\n"+profile)
	}

	memory, err := p.ReadMemory()
	if err != nil {
		slog.Warn("PG: failed to read memory for system prompt", "error", err)
	} else if memory != "" {
		parts = append(parts, "## Saved Memory\nThe following facts have been saved from previous conversations with this user.\n"+memory)
	}

	return strings.Join(parts, "\n\n")
}

func (p *PGMemoryProvider) Shutdown() error {
	return nil
}
