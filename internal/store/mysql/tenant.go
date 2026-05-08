package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myTenantStore struct{ db *sql.DB }

func (s *myTenantStore) Create(ctx context.Context, t *store.Tenant) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Plan, t.RateLimitRPM, t.MaxSessions, now, now)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	return nil
}

func (s *myTenantStore) Get(ctx context.Context, id string) (*store.Tenant, error) {
	t := &store.Tenant{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at
		 FROM tenants WHERE id = ? AND deleted_at IS NULL`, id).
		Scan(&t.ID, &t.Name, &t.Plan, &t.RateLimitRPM, &t.MaxSessions, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return t, nil
}

func (s *myTenantStore) Update(ctx context.Context, t *store.Tenant) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET name=?, plan=?, rate_limit_rpm=?, max_sessions=?, updated_at=NOW()
		 WHERE id=? AND deleted_at IS NULL`,
		t.Name, t.Plan, t.RateLimitRPM, t.MaxSessions, t.ID)
	return err
}

func (s *myTenantStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete tenant: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tenant not found or already deleted")
	}
	return nil
}

func (s *myTenantStore) List(ctx context.Context, opts store.ListOptions) ([]*store.Tenant, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tenants: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at
		 FROM tenants WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tenants []*store.Tenant
	for rows.Next() {
		t := &store.Tenant{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Plan, &t.RateLimitRPM, &t.MaxSessions, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tenants = append(tenants, t)
	}
	return tenants, total, rows.Err()
}

func (s *myTenantStore) ListDeleted(ctx context.Context, olderThan time.Time) ([]*store.Tenant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at, deleted_at
		 FROM tenants WHERE deleted_at IS NOT NULL AND deleted_at < ? ORDER BY deleted_at ASC`, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*store.Tenant
	for rows.Next() {
		t := &store.Tenant{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Plan, &t.RateLimitRPM, &t.MaxSessions, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

func (s *myTenantStore) HardDelete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = ?`, id)
	return err
}

func (s *myTenantStore) Restore(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tenant not found or not deleted")
	}
	return nil
}

var _ store.TenantStore = (*myTenantStore)(nil)
