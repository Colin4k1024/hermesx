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

	rows, err := p.pool.Query(ctx,
		`SELECT key, content FROM memories
		 WHERE tenant_id = $1 AND user_id = $2
		 ORDER BY updated_at DESC`,
		p.tenantID, p.userID)
	if err != nil {
		return "", fmt.Errorf("pg read memory: %w", err)
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var key, content string
		if err := rows.Scan(&key, &content); err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n%s", key, content))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("pg iterate memories: %w", err)
	}

	return strings.Join(parts, "\n\n"), nil
}

func (p *PGMemoryProvider) SaveMemory(key, content string) error {
	if key == "" || content == "" {
		return fmt.Errorf("both key and content are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := p.pool.Exec(ctx,
		`INSERT INTO memories (tenant_id, user_id, key, content, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, user_id, key)
		 DO UPDATE SET content = $4, updated_at = now()`,
		p.tenantID, p.userID, key, content)
	if err != nil {
		return fmt.Errorf("pg save memory: %w", err)
	}
	return nil
}

func (p *PGMemoryProvider) DeleteMemory(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tag, err := p.pool.Exec(ctx,
		`DELETE FROM memories WHERE tenant_id = $1 AND user_id = $2 AND key = $3`,
		p.tenantID, p.userID, key)
	if err != nil {
		return fmt.Errorf("pg delete memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory key '%s' not found", key)
	}
	return nil
}

func (p *PGMemoryProvider) ReadUserProfile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var content string
	err := p.pool.QueryRow(ctx,
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

	_, err := p.pool.Exec(ctx,
		`INSERT INTO user_profiles (tenant_id, user_id, content, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (tenant_id, user_id)
		 DO UPDATE SET content = $3, updated_at = now()`,
		p.tenantID, p.userID, content)
	if err != nil {
		return fmt.Errorf("pg save user profile: %w", err)
	}
	return nil
}

func (p *PGMemoryProvider) SystemPromptBlock() string {
	var parts []string

	memory, err := p.ReadMemory()
	if err != nil {
		slog.Warn("PG: failed to read memory for system prompt", "error", err)
	} else if memory != "" {
		parts = append(parts, "## Agent Memory\n"+memory)
	}

	profile, err := p.ReadUserProfile()
	if err != nil {
		slog.Warn("PG: failed to read user profile for system prompt", "error", err)
	} else if profile != "" {
		parts = append(parts, "## User Profile\n"+profile)
	}

	return strings.Join(parts, "\n\n")
}

func (p *PGMemoryProvider) Shutdown() error {
	return nil
}
