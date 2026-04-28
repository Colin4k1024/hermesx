package pg

import (
	"context"
	"fmt"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgTenantStore struct {
	pool *pgxpool.Pool
}

func (s *pgTenantStore) Create(ctx context.Context, t *store.Tenant) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO tenants (id, name, plan, rate_limit_rpm, max_sessions) VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5)`,
		t.ID, t.Name, t.Plan, t.RateLimitRPM, t.MaxSessions,
	)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (s *pgTenantStore) Get(ctx context.Context, id string) (*store.Tenant, error) {
	t := &store.Tenant{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Plan, &t.RateLimitRPM, &t.MaxSessions, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return t, nil
}

func (s *pgTenantStore) Update(ctx context.Context, t *store.Tenant) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE tenants SET name=$2, plan=$3, rate_limit_rpm=$4, max_sessions=$5, updated_at=now() WHERE id=$1`,
		t.ID, t.Name, t.Plan, t.RateLimitRPM, t.MaxSessions,
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	return nil
}

func (s *pgTenantStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	return nil
}

func (s *pgTenantStore) List(ctx context.Context, opts store.ListOptions) ([]*store.Tenant, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tenants: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, name, plan, rate_limit_rpm, max_sessions, created_at, updated_at FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, opts.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*store.Tenant
	for rows.Next() {
		t := &store.Tenant{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Plan, &t.RateLimitRPM, &t.MaxSessions, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tenants: %w", err)
	}
	return tenants, total, nil
}

var _ store.TenantStore = (*pgTenantStore)(nil)
