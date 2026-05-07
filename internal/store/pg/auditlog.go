package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAuditLogStore struct {
	pool *pgxpool.Pool
}

func (s *pgAuditLogStore) Append(ctx context.Context, log *store.AuditLog) error {
	var userID pgtype.UUID
	if log.UserID != "" {
		if parsed, err := uuid.Parse(log.UserID); err == nil {
			userID.Valid = true
			userID.Bytes = parsed
		}
	}

	// tenant_id is nullable for auth failure events (no tenant context).
	var tenantID pgtype.UUID
	if log.TenantID != "" {
		if parsed, err := uuid.Parse(log.TenantID); err == nil {
			tenantID.Valid = true
			tenantID.Bytes = parsed
		}
	}

	const q = `INSERT INTO audit_logs (tenant_id, user_id, session_id, action, detail, request_id, status_code, latency_ms, source_ip, error_code, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	args := []any{tenantID, userID, log.SessionID, log.Action, log.Detail, log.RequestID, log.StatusCode, log.LatencyMs,
		log.SourceIP, log.ErrorCode, log.UserAgent}

	if log.TenantID != "" {
		return withTenantTx(ctx, s.pool, log.TenantID, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, q, args...)
			if err != nil {
				return fmt.Errorf("append audit log: %w", err)
			}
			return nil
		})
	}

	// Auth failure events have no tenant context — direct exec without SET LOCAL.
	// Safety: RLS WITH CHECK uses current_setting('app.current_tenant', false) which
	// errors when unset, so any accidental call with a real tenantID would fail at DB level.
	_, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func (s *pgAuditLogStore) List(ctx context.Context, tenantID string, opts store.AuditListOptions) ([]*store.AuditLog, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, COALESCE(tenant_id::text, ''), COALESCE(user_id::text, ''), session_id, action, detail, request_id, status_code, latency_ms, COALESCE(source_ip, ''), COALESCE(error_code, ''), COALESCE(user_agent, ''), created_at FROM audit_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []any{}
	argIdx := 1

	if tenantID != "" {
		query += fmt.Sprintf(` AND tenant_id = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND tenant_id = $%d`, argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if opts.Action != "" {
		query += fmt.Sprintf(` AND action = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND action = $%d`, argIdx)
		args = append(args, opts.Action)
		argIdx++
	}

	if opts.From != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *opts.From)
		argIdx++
	}

	if opts.To != nil {
		query += fmt.Sprintf(` AND created_at < $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND created_at < $%d`, argIdx)
		args = append(args, *opts.To)
		argIdx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, opts.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*store.AuditLog
	for rows.Next() {
		l := &store.AuditLog{}
		if err := rows.Scan(&l.ID, &l.TenantID, &l.UserID, &l.SessionID, &l.Action, &l.Detail, &l.RequestID, &l.StatusCode, &l.LatencyMs, &l.SourceIP, &l.ErrorCode, &l.UserAgent, &l.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}
	return logs, total, nil
}

// DeleteByTenant removes all audit logs for a tenant (used during hard delete).
func (s *pgAuditLogStore) DeleteByTenant(ctx context.Context, tenantID string) (int64, error) {
	var affected int64
	err := withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM audit_logs WHERE tenant_id = $1`, tenantID)
		if err != nil {
			return fmt.Errorf("delete audit logs by tenant: %w", err)
		}
		affected = tag.RowsAffected()
		return nil
	})
	return affected, err
}

var _ store.AuditLogStore = (*pgAuditLogStore)(nil)
