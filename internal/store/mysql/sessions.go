package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type mySessionStore struct{ db *sql.DB }

func (s *mySessionStore) Create(ctx context.Context, tenantID string, sess *store.Session) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, tenant_id, platform, user_id, model, system_prompt, parent_session_id, title, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, tenantID, sess.Platform, sess.UserID, sess.Model,
		nullStr(sess.SystemPrompt), nullStr(sess.ParentSessionID), nullStr(sess.Title), sess.StartedAt)
	return err
}

func (s *mySessionStore) Get(ctx context.Context, tenantID, sessionID string) (*store.Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, platform, user_id, model,
		       COALESCE(system_prompt,''), COALESCE(parent_session_id,''),
		       COALESCE(title,''), started_at, ended_at,
		       COALESCE(end_reason,''), message_count, tool_call_count,
		       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd
		FROM sessions WHERE id = ? AND tenant_id = ?`, sessionID, tenantID)

	sess := &store.Session{}
	err := row.Scan(
		&sess.ID, &sess.TenantID, &sess.Platform, &sess.UserID, &sess.Model,
		&sess.SystemPrompt, &sess.ParentSessionID, &sess.Title,
		&sess.StartedAt, &sess.EndedAt, &sess.EndReason,
		&sess.MessageCount, &sess.ToolCallCount,
		&sess.InputTokens, &sess.OutputTokens, &sess.CacheReadTokens, &sess.CacheWriteTokens,
		&sess.EstimatedCostUSD)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *mySessionStore) End(ctx context.Context, tenantID, sessionID, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET ended_at = ?, end_reason = ? WHERE tenant_id = ? AND id = ?`,
		time.Now(), reason, tenantID, sessionID)
	return err
}

func (s *mySessionStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.Session, int, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE tenant_id = ?`, tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, platform, user_id, model, COALESCE(title,''), started_at, ended_at,
		       message_count, input_tokens, output_tokens, estimated_cost_usd
		FROM sessions WHERE tenant_id = ?
		ORDER BY started_at DESC LIMIT ? OFFSET ?`, tenantID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sessions []*store.Session
	for rows.Next() {
		sess := &store.Session{}
		if err := rows.Scan(&sess.ID, &sess.TenantID, &sess.Platform, &sess.UserID, &sess.Model, &sess.Title,
			&sess.StartedAt, &sess.EndedAt, &sess.MessageCount, &sess.InputTokens, &sess.OutputTokens,
			&sess.EstimatedCostUSD); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, total, rows.Err()
}

func (s *mySessionStore) Delete(ctx context.Context, tenantID, sessionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE tenant_id = ? AND session_id = ?`, tenantID, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE tenant_id = ? AND id = ?`, tenantID, sessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *mySessionStore) UpdateTokens(ctx context.Context, tenantID, sessionID string, delta store.TokenDelta) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			cache_read_tokens = cache_read_tokens + ?,
			cache_write_tokens = cache_write_tokens + ?,
			message_count = message_count + 1
		WHERE tenant_id = ? AND id = ?`,
		delta.Input, delta.Output, delta.CacheRead, delta.CacheWrite, tenantID, sessionID)
	return err
}

func (s *mySessionStore) SetTitle(ctx context.Context, tenantID, sessionID, title string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET title = ? WHERE tenant_id = ? AND id = ?`,
		title, tenantID, sessionID)
	return err
}

var _ store.SessionStore = (*mySessionStore)(nil)
