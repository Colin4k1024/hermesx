package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myExecutionReceiptStore struct{ db *sql.DB }

func (s *myExecutionReceiptStore) Create(ctx context.Context, receipt *store.ExecutionReceipt) error {
	if receipt.ID == "" {
		receipt.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO execution_receipts (id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, idempotency_id, trace_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		receipt.ID, receipt.TenantID, receipt.SessionID, receipt.UserID,
		receipt.ToolName, receipt.Input, receipt.Output,
		receipt.Status, receipt.DurationMs,
		nullStr(receipt.IdempotencyID), nullStr(receipt.TraceID))
	if err != nil {
		return fmt.Errorf("create execution receipt: %w", err)
	}
	return s.db.QueryRowContext(ctx, `SELECT created_at FROM execution_receipts WHERE id = ?`, receipt.ID).
		Scan(&receipt.CreatedAt)
}

func (s *myExecutionReceiptStore) Get(ctx context.Context, tenantID, id string) (*store.ExecutionReceipt, error) {
	r := &store.ExecutionReceipt{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms,
		        COALESCE(idempotency_id,''), COALESCE(trace_id,''), created_at
		 FROM execution_receipts WHERE tenant_id = ? AND id = ?`, tenantID, id).
		Scan(&r.ID, &r.TenantID, &r.SessionID, &r.UserID, &r.ToolName, &r.Input, &r.Output,
			&r.Status, &r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt: %w", err)
	}
	return r, nil
}

func (s *myExecutionReceiptStore) List(ctx context.Context, tenantID string, opts store.ReceiptListOptions) ([]*store.ExecutionReceipt, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, COALESCE(idempotency_id,''), COALESCE(trace_id,''), created_at FROM execution_receipts WHERE tenant_id = ?`
	countQuery := `SELECT COUNT(*) FROM execution_receipts WHERE tenant_id = ?`
	args := []any{tenantID}
	countArgs := []any{tenantID}

	if opts.SessionID != "" {
		query += ` AND session_id = ?`
		countQuery += ` AND session_id = ?`
		args = append(args, opts.SessionID)
		countArgs = append(countArgs, opts.SessionID)
	}
	if opts.ToolName != "" {
		query += ` AND tool_name = ?`
		countQuery += ` AND tool_name = ?`
		args = append(args, opts.ToolName)
		countArgs = append(countArgs, opts.ToolName)
	}
	if opts.Status != "" {
		query += ` AND status = ?`
		countQuery += ` AND status = ?`
		args = append(args, opts.Status)
		countArgs = append(countArgs, opts.Status)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count execution receipts: %w", err)
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var receipts []*store.ExecutionReceipt
	for rows.Next() {
		r := &store.ExecutionReceipt{}
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SessionID, &r.UserID, &r.ToolName, &r.Input, &r.Output,
			&r.Status, &r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt); err != nil {
			return nil, 0, err
		}
		receipts = append(receipts, r)
	}
	return receipts, total, rows.Err()
}

func (s *myExecutionReceiptStore) GetByIdempotencyID(ctx context.Context, tenantID, idempotencyID string) (*store.ExecutionReceipt, error) {
	r := &store.ExecutionReceipt{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms,
		        COALESCE(idempotency_id,''), COALESCE(trace_id,''), created_at
		 FROM execution_receipts WHERE tenant_id = ? AND idempotency_id = ?`, tenantID, idempotencyID).
		Scan(&r.ID, &r.TenantID, &r.SessionID, &r.UserID, &r.ToolName, &r.Input, &r.Output,
			&r.Status, &r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt by idempotency_id: %w", err)
	}
	return r, nil
}

var _ store.ExecutionReceiptStore = (*myExecutionReceiptStore)(nil)
