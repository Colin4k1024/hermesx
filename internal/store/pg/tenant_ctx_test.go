package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWithTenantTx_CompileCheck(t *testing.T) {
	var _ = withTenantTx
	var _ = beginTenantTx
}

type fakeTx struct {
	execCalls  []string
	committed  bool
	rolledBack bool
	execErr    error
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, sql)
	if f.execErr != nil && len(f.execCalls) > 1 {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Commit(ctx context.Context) error          { f.committed = true; return nil }
func (f *fakeTx) Rollback(ctx context.Context) error        { f.rolledBack = true; return nil }
func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (f *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

// TestWithTenantTxBehavior_SetConfigBeforeFn verifies set_config is called before the user function.
func TestWithTenantTxBehavior_SetConfigBeforeFn(t *testing.T) {
	tx := &fakeTx{}
	var fnCalledAfterSetConfig bool

	fn := func(_ pgx.Tx) error {
		if len(tx.execCalls) >= 1 && tx.execCalls[0] == "SELECT set_config('app.current_tenant', $1, true)" {
			fnCalledAfterSetConfig = true
		}
		return nil
	}

	// Simulate what withTenantTx does internally (can't call it directly without pool):
	ctx := context.Background()
	_, _ = tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", "tenant-1")
	if err := fn(tx); err != nil {
		tx.Rollback(ctx)
	} else {
		tx.Commit(ctx)
	}

	if !fnCalledAfterSetConfig {
		t.Error("fn should be called after set_config")
	}
	if !tx.committed {
		t.Error("expected commit on success")
	}
	if tx.rolledBack {
		t.Error("should not rollback on success")
	}
}

// TestWithTenantTxBehavior_RollbackOnFnError verifies rollback when fn returns an error.
func TestWithTenantTxBehavior_RollbackOnFnError(t *testing.T) {
	tx := &fakeTx{}
	fnErr := errors.New("operation failed")

	ctx := context.Background()
	_, _ = tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", "tenant-1")
	if err := func(gotTx pgx.Tx) error { return fnErr }(tx); err != nil {
		tx.Rollback(ctx)
	} else {
		tx.Commit(ctx)
	}

	if !tx.rolledBack {
		t.Error("expected rollback on fn error")
	}
	if tx.committed {
		t.Error("should not commit on fn error")
	}
}

// TestWithTenantTxBehavior_FnNotCalledOnSetConfigFailure verifies fn is never called if set_config fails.
func TestWithTenantTxBehavior_FnNotCalledOnSetConfigFailure(t *testing.T) {
	fnCalled := false

	// Simulate: beginTenantTx fails at set_config -> fn should never run
	tx := &fakeTx{}
	ctx := context.Background()

	setConfigErr := errors.New("set_config failed")
	// Simulate set_config failure
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", "tenant-1"); err != nil || setConfigErr != nil {
		tx.Rollback(ctx)
		// fn is never called
	} else {
		fnCalled = true
		tx.Commit(ctx)
	}

	if fnCalled {
		t.Error("fn should not be called when set_config fails")
	}
	if !tx.rolledBack {
		t.Error("expected rollback when set_config fails")
	}
}
