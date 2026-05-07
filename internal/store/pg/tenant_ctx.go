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

// withTenantTx wraps a write operation in a transaction with the RLS tenant context set.
// The fn receives the transaction; if fn returns nil the tx is committed, otherwise rolled back.
func withTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx) error) error {
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
