package pg

import (
	"context"
	"errors"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSessionStore struct{ pool *pgxpool.Pool }

func (s *pgSessionStore) Create(ctx context.Context, tenantID string, sess *store.Session) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO sessions (id, tenant_id, platform, user_id, model, system_prompt, parent_session_id, title, started_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			sess.ID, tenantID, sess.Platform, sess.UserID, sess.Model,
			sess.SystemPrompt, sess.ParentSessionID, sess.Title, sess.StartedAt)
		return err
	})
}

func (s *pgSessionStore) Get(ctx context.Context, tenantID, sessionID string) (*store.Session, error) {
	// Set tenant context so the cmd=ALL isolation policy allows the SELECT.
	var row pgx.Rows
	err := withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		var txErr error
		row, txErr = tx.Query(ctx, `
			SELECT id, tenant_id, platform, user_id, model, system_prompt, parent_session_id,
			       title, started_at, ended_at, end_reason, message_count, tool_call_count,
			       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd
			FROM sessions WHERE id = $1 AND tenant_id = $2`, sessionID, tenantID)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	defer row.Close()
	if !row.Next() {
		return nil, nil // not found (RLS returns empty set when tenant mismatch)
	}

	sess := &store.Session{}
	var costUSD *float64
	var systemPrompt, parentSessionID, title, endReason any
	scanErr := row.Scan(
		&sess.ID, &sess.TenantID, &sess.Platform, &sess.UserID, &sess.Model,
		&systemPrompt, &parentSessionID, &title, &sess.StartedAt,
		&sess.EndedAt, &endReason, &sess.MessageCount, &sess.ToolCallCount,
		&sess.InputTokens, &sess.OutputTokens, &sess.CacheReadTokens, &sess.CacheWriteTokens,
		&costUSD)
	// Assign nullable string fields after scan (NULL → empty string, not error).
	// pgx returns NOT NULL columns as plain string, NULL columns as nil.
	if systemPrompt != nil {
		if ps, ok := systemPrompt.(string); ok {
			sess.SystemPrompt = ps
		}
	}
	if parentSessionID != nil {
		if ps, ok := parentSessionID.(string); ok {
			sess.ParentSessionID = ps
		}
	}
	if title != nil {
		if t, ok := title.(string); ok {
			sess.Title = t
		}
	}
	if endReason != nil {
		if er, ok := endReason.(string); ok {
			sess.EndReason = er
		}
	}
	if scanErr != nil {
		if errors.Is(scanErr, context.DeadlineExceeded) {
			return nil, scanErr
		}
		return nil, nil // not found
	}
	if costUSD != nil {
		sess.EstimatedCostUSD = *costUSD
	}
	return sess, nil
}

func (s *pgSessionStore) End(ctx context.Context, tenantID, sessionID, reason string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE sessions SET ended_at = $1, end_reason = $2 WHERE tenant_id = $3 AND id = $4`,
			time.Now(), reason, tenantID, sessionID)
		return err
	})
}

func (s *pgSessionStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.Session, int, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	var sessions []*store.Session
	var total int

	err := withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		countRow := tx.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE tenant_id = $1`, tenantID)
		if err := countRow.Scan(&total); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
			SELECT id, tenant_id, platform, user_id, model, title, started_at, ended_at,
			       message_count, input_tokens, output_tokens, estimated_cost_usd
			FROM sessions WHERE tenant_id = $1
			ORDER BY started_at DESC LIMIT $2 OFFSET $3`, tenantID, opts.Limit, opts.Offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s := &store.Session{}
			var costUSD *float64
			var title any
			if err := rows.Scan(&s.ID, &s.TenantID, &s.Platform, &s.UserID, &s.Model, &title,
				&s.StartedAt, &s.EndedAt, &s.MessageCount, &s.InputTokens, &s.OutputTokens,
				&costUSD); err != nil {
				continue
			}
			if title != nil {
				if t, ok := title.(string); ok {
					s.Title = t
				}
			}
			if costUSD != nil {
				s.EstimatedCostUSD = *costUSD
			}
			sessions = append(sessions, s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

func (s *pgSessionStore) Delete(ctx context.Context, tenantID, sessionID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM messages WHERE tenant_id = $1 AND session_id = $2`, tenantID, sessionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM sessions WHERE tenant_id = $1 AND id = $2`, tenantID, sessionID)
		return err
	})
}

func (s *pgSessionStore) UpdateTokens(ctx context.Context, tenantID, sessionID string, delta store.TokenDelta) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE sessions SET
				input_tokens = input_tokens + $1,
				output_tokens = output_tokens + $2,
				cache_read_tokens = cache_read_tokens + $3,
				cache_write_tokens = cache_write_tokens + $4,
				message_count = message_count + 1
			WHERE tenant_id = $5 AND id = $6`,
			delta.Input, delta.Output, delta.CacheRead, delta.CacheWrite, tenantID, sessionID)
		return err
	})
}

func (s *pgSessionStore) SetTitle(ctx context.Context, tenantID, sessionID, title string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE sessions SET title = $1 WHERE tenant_id = $2 AND id = $3`,
			title, tenantID, sessionID)
		return err
	})
}
