package egress

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RuleAdminStore interface {
	RuleStore
	CreateRule(ctx context.Context, r EgressRule) (string, error)
	DeleteRule(ctx context.Context, id string, tenantID string) error
	ListRules(ctx context.Context, tenantID string) ([]EgressRule, error)
}

type EmptyRuleStore struct{}

func (EmptyRuleStore) LoadRules(_ context.Context, _ string) ([]EgressRule, error) {
	return nil, nil
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) LoadRules(ctx context.Context, tenantID string) ([]EgressRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, host_pattern, path_prefix, action, priority
		 FROM egress_rules WHERE tenant_id = $1 ORDER BY priority DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []EgressRule
	for rows.Next() {
		var r EgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.HostPattern, &r.PathPrefix, &r.Action, &r.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *PostgresStore) CreateRule(ctx context.Context, r EgressRule) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO egress_rules (tenant_id, host_pattern, path_prefix, action, priority)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		r.TenantID, r.HostPattern, r.PathPrefix, r.Action, r.Priority).Scan(&id)
	return id, err
}

func (s *PostgresStore) DeleteRule(ctx context.Context, id string, tenantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM egress_rules WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}

func (s *PostgresStore) ListRules(ctx context.Context, tenantID string) ([]EgressRule, error) {
	query := `SELECT id, tenant_id, host_pattern, path_prefix, action, priority FROM egress_rules`
	var args []any

	if tenantID != "" {
		query += ` WHERE tenant_id = $1`
		args = append(args, tenantID)
	}
	query += ` ORDER BY priority DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []EgressRule
	for rows.Next() {
		var r EgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.HostPattern, &r.PathPrefix, &r.Action, &r.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

type PGXStore struct {
	pool *pgxpool.Pool
}

func NewPGXStore(pool *pgxpool.Pool) *PGXStore {
	return &PGXStore{pool: pool}
}

func (s *PGXStore) LoadRules(ctx context.Context, tenantID string) ([]EgressRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, tenant_id::text, host_pattern, path_prefix, action, priority
		 FROM egress_rules WHERE tenant_id::text = $1 ORDER BY priority DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []EgressRule
	for rows.Next() {
		var r EgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.HostPattern, &r.PathPrefix, &r.Action, &r.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *PGXStore) CreateRule(ctx context.Context, r EgressRule) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO egress_rules (tenant_id, host_pattern, path_prefix, action, priority)
		 VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		r.TenantID, r.HostPattern, r.PathPrefix, r.Action, r.Priority).Scan(&id)
	return id, err
}

func (s *PGXStore) DeleteRule(ctx context.Context, id string, tenantID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM egress_rules WHERE id = $1::uuid AND tenant_id = $2::uuid`, id, tenantID)
	return err
}

func (s *PGXStore) ListRules(ctx context.Context, tenantID string) ([]EgressRule, error) {
	query := `SELECT id::text, tenant_id::text, host_pattern, path_prefix, action, priority FROM egress_rules`
	var args []any

	if tenantID != "" {
		query += ` WHERE tenant_id::text = $1`
		args = append(args, tenantID)
	}
	query += ` ORDER BY priority DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []EgressRule
	for rows.Next() {
		var r EgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.HostPattern, &r.PathPrefix, &r.Action, &r.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
