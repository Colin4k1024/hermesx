package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// beginTenantTx starts a transaction and sets the RLS tenant context variable.
// All write operations (INSERT/UPDATE/DELETE) must use this to satisfy
// WITH CHECK policies that reference current_setting('app.current_tenant', false).
//
// # RLS architecture overview
//
// Tenant isolation is enforced at the database layer via Row-Level Security (RLS):
//   - Every tenant-scoped table has a USING policy: tenant_id = current_setting('app.current_tenant').
//   - Every write table has a WITH CHECK policy for the same expression.
//   - This file sets the session-local config variable before any DML so policies pass.
//
// # Admin / cross-tenant queries
//
// Admin endpoints that need to read across all tenants (e.g. aggregate usage, audit log
// cross-tenant search) intentionally bypass tenant context in two ways:
//
//  1. Tables explicitly excluded from RLS (e.g. audit_logs in the purge schema) are queried
//     directly from the pool without setting app.current_tenant.
//
//  2. Tables that ARE subject to RLS require the application DB user to hold the BYPASSRLS
//     privilege (or SUPERUSER), then the admin handler can run queries without setting
//     app.current_tenant — RLS policies are skipped entirely for that role.
//     If BYPASSRLS is not granted, wrap the query in a transaction and execute:
//     SET LOCAL row_security = off;
//     before querying.
//
// Code paths that bypass tenant context must only be reachable via admin-scoped routes
// (RequireScope("admin") middleware). Application-level filtering is an additional defense
// in depth layer, not a replacement for database-level isolation.
func beginTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
		tx.Rollback(ctx) //nolint:errcheck
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	return tx, nil
}

// WithTenantTx wraps a write operation in a transaction with the RLS tenant context set.
// The fn receives the transaction; if fn returns nil the tx is committed, otherwise rolled back.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := beginTenantTx(ctx, pool, tenantID)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback(ctx) //nolint:errcheck
		return err
	}
	return tx.Commit(ctx)
}

// withTenantTx is a package-local alias for WithTenantTx.
var withTenantTx = WithTenantTx
