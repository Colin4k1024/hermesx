package pg

import (
	"context"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgExecutionReceiptStore struct {
	pool *pgxpool.Pool
}

func (s *pgExecutionReceiptStore) Create(ctx context.Context, receipt *store.ExecutionReceipt) error {
	return withTenantTx(ctx, s.pool, receipt.TenantID, func(tx pgx.Tx) error {
		const q = `INSERT INTO execution_receipts (tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, idempotency_id, trace_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), NULLIF($10, ''))
			RETURNING id, created_at`
		return tx.QueryRow(ctx, q,
			receipt.TenantID, receipt.SessionID, receipt.UserID,
			receipt.ToolName, receipt.Input, receipt.Output,
			receipt.Status, receipt.DurationMs,
			receipt.IdempotencyID, receipt.TraceID,
		).Scan(&receipt.ID, &receipt.CreatedAt)
	})
}

func (s *pgExecutionReceiptStore) Get(ctx context.Context, tenantID, id string) (*store.ExecutionReceipt, error) {
	const q = `SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, COALESCE(idempotency_id, ''), COALESCE(trace_id, ''), created_at
		FROM execution_receipts WHERE tenant_id = $1 AND id = $2`
	r := &store.ExecutionReceipt{}
	err := s.pool.QueryRow(ctx, q, tenantID, id).Scan(
		&r.ID, &r.TenantID, &r.SessionID, &r.UserID,
		&r.ToolName, &r.Input, &r.Output, &r.Status,
		&r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt: %w", err)
	}
	return r, nil
}

func (s *pgExecutionReceiptStore) List(ctx context.Context, tenantID string, opts store.ReceiptListOptions) ([]*store.ExecutionReceipt, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, COALESCE(idempotency_id, ''), COALESCE(trace_id, ''), created_at FROM execution_receipts WHERE tenant_id = $1`
	countQuery := `SELECT COUNT(*) FROM execution_receipts WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if opts.SessionID != "" {
		query += fmt.Sprintf(` AND session_id = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND session_id = $%d`, argIdx)
		args = append(args, opts.SessionID)
		argIdx++
	}
	if opts.ToolName != "" {
		query += fmt.Sprintf(` AND tool_name = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND tool_name = $%d`, argIdx)
		args = append(args, opts.ToolName)
		argIdx++
	}
	if opts.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, opts.Status)
		argIdx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count execution receipts: %w", err)
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, opts.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list execution receipts: %w", err)
	}
	defer rows.Close()

	var receipts []*store.ExecutionReceipt
	for rows.Next() {
		r := &store.ExecutionReceipt{}
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SessionID, &r.UserID, &r.ToolName, &r.Input, &r.Output, &r.Status, &r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan execution receipt: %w", err)
		}
		receipts = append(receipts, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate execution receipts: %w", err)
	}
	return receipts, total, nil
}

func (s *pgExecutionReceiptStore) GetByIdempotencyID(ctx context.Context, tenantID, idempotencyID string) (*store.ExecutionReceipt, error) {
	const q = `SELECT id, tenant_id, session_id, user_id, tool_name, input, output, status, duration_ms, COALESCE(idempotency_id, ''), COALESCE(trace_id, ''), created_at
		FROM execution_receipts WHERE tenant_id = $1 AND idempotency_id = $2`
	r := &store.ExecutionReceipt{}
	err := s.pool.QueryRow(ctx, q, tenantID, idempotencyID).Scan(
		&r.ID, &r.TenantID, &r.SessionID, &r.UserID,
		&r.ToolName, &r.Input, &r.Output, &r.Status,
		&r.DurationMs, &r.IdempotencyID, &r.TraceID, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get execution receipt by idempotency_id: %w", err)
	}
	return r, nil
}

var _ store.ExecutionReceiptStore = (*pgExecutionReceiptStore)(nil)
