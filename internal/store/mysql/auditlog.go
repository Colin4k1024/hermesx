package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myAuditLogStore struct{ db *sql.DB }

func (s *myAuditLogStore) Append(ctx context.Context, log *store.AuditLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_logs (tenant_id, user_id, session_id, action, detail, request_id, status_code, latency_ms, source_ip, error_code, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullStr(log.TenantID), nullStr(log.UserID), log.SessionID, log.Action, log.Detail,
		log.RequestID, log.StatusCode, log.LatencyMs, log.SourceIP, log.ErrorCode, log.UserAgent)
	return err
}

func (s *myAuditLogStore) List(ctx context.Context, tenantID string, opts store.AuditListOptions) ([]*store.AuditLog, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, COALESCE(tenant_id,''), COALESCE(user_id,''), COALESCE(session_id,''), action, COALESCE(detail,''), COALESCE(request_id,''), COALESCE(status_code,0), COALESCE(latency_ms,0), COALESCE(source_ip,''), COALESCE(error_code,''), COALESCE(user_agent,''), created_at FROM audit_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []any{}
	countArgs := []any{}

	if tenantID != "" {
		query += ` AND tenant_id = ?`
		countQuery += ` AND tenant_id = ?`
		args = append(args, tenantID)
		countArgs = append(countArgs, tenantID)
	}
	if opts.Action != "" {
		query += ` AND action = ?`
		countQuery += ` AND action = ?`
		args = append(args, opts.Action)
		countArgs = append(countArgs, opts.Action)
	}
	if opts.From != nil {
		query += ` AND created_at >= ?`
		countQuery += ` AND created_at >= ?`
		args = append(args, *opts.From)
		countArgs = append(countArgs, *opts.From)
	}
	if opts.To != nil {
		query += ` AND created_at < ?`
		countQuery += ` AND created_at < ?`
		args = append(args, *opts.To)
		countArgs = append(countArgs, *opts.To)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*store.AuditLog
	for rows.Next() {
		l := &store.AuditLog{}
		if err := rows.Scan(&l.ID, &l.TenantID, &l.UserID, &l.SessionID, &l.Action, &l.Detail, &l.RequestID,
			&l.StatusCode, &l.LatencyMs, &l.SourceIP, &l.ErrorCode, &l.UserAgent, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (s *myAuditLogStore) DeleteByTenant(ctx context.Context, tenantID string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_logs WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *myAuditLogStore) ArchiveOlderThan(ctx context.Context, cutoff time.Time, batchSize int) ([]*store.AuditLog, error) {
	batchSize = normalizeAuditArchiveBatchSize(batchSize)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin audit archival tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// tenant_sql_check:skip — audit archival is a privileged global retention job, not a tenant request path.
	const selectQ = `SELECT id, COALESCE(tenant_id,''), COALESCE(user_id,''), COALESCE(session_id,''), action, COALESCE(detail,''), COALESCE(request_id,''), COALESCE(status_code,0), COALESCE(latency_ms,0), COALESCE(source_ip,''), COALESCE(error_code,''), COALESCE(user_agent,''), created_at
FROM audit_logs
WHERE created_at < ?
ORDER BY created_at ASC
LIMIT ? FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, selectQ, cutoff, batchSize)
	if err != nil {
		return nil, fmt.Errorf("select archive audit logs: %w", err)
	}

	var logs []*store.AuditLog
	var ids []int64
	for rows.Next() {
		l := &store.AuditLog{}
		if err := rows.Scan(&l.ID, &l.TenantID, &l.UserID, &l.SessionID, &l.Action, &l.Detail, &l.RequestID,
			&l.StatusCode, &l.LatencyMs, &l.SourceIP, &l.ErrorCode, &l.UserAgent, &l.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan archived audit log: %w", err)
		}
		logs = append(logs, l)
		ids = append(ids, l.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate archived audit logs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close archived audit log rows: %w", err)
	}

	if len(ids) > 0 {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		// tenant_sql_check:skip — deletes only the IDs locked by the global archival transaction above.
		deleteQ := `DELETE FROM audit_logs WHERE id IN (` + mysqlPlaceholders(len(ids)) + `)`
		res, err := tx.ExecContext(ctx, deleteQ, args...)
		if err != nil {
			return nil, fmt.Errorf("delete archived audit logs: %w", err)
		}
		if affected, err := res.RowsAffected(); err == nil && affected != int64(len(ids)) {
			return nil, fmt.Errorf("delete archived audit logs: affected %d, want %d", affected, len(ids))
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit audit archival tx: %w", err)
	}
	committed = true
	return logs, nil
}

func (s *myAuditLogStore) ArchiveCount(ctx context.Context, cutoff time.Time) (int64, error) {
	var count int64
	// tenant_sql_check:skip — audit archival planning intentionally counts across all tenants.
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE created_at < ?`, cutoff).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count archivable audit logs: %w", err)
	}
	return count, nil
}

func normalizeAuditArchiveBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return 1000
	}
	if batchSize > 10000 {
		return 10000
	}
	return batchSize
}

func mysqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

var _ store.AuditLogStore = (*myAuditLogStore)(nil)
