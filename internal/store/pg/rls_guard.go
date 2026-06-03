package pg

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// verifySuperuserSafety checks that the database connection user is NOT a superuser.
// RLS policies are entirely bypassed for superuser connections, which breaks
// multi-tenant isolation. This check runs once at pool startup and fails hard
// unless explicitly overridden via HERMESX_ALLOW_SUPERUSER=true (for dev/test only).
func verifySuperuserSafety(ctx context.Context, pool *pgxpool.Pool) error {
	var isSuperuser bool
	err := pool.QueryRow(ctx,
		"SELECT rolsuper FROM pg_roles WHERE rolname = current_user",
	).Scan(&isSuperuser)
	if err != nil {
		return fmt.Errorf("rls guard: failed to check superuser status: %w", err)
	}

	if !isSuperuser {
		slog.Info("rls guard: connection user is non-superuser, RLS is active")
		return nil
	}

	if os.Getenv("HERMESX_ALLOW_SUPERUSER") == "true" {
		slog.Warn("rls guard: connection user is SUPERUSER — RLS policies are BYPASSED",
			"override", "HERMESX_ALLOW_SUPERUSER=true",
			"risk", "multi-tenant isolation depends solely on application logic",
		)
		return nil
	}

	return fmt.Errorf(
		"rls guard: FATAL — database connection user has SUPERUSER privilege. " +
			"RLS policies are completely bypassed, multi-tenant isolation is BROKEN. " +
			"Use a non-superuser role (GRANT connect, usage ON ...) or set " +
			"HERMESX_ALLOW_SUPERUSER=true to override (development only)",
	)
}

// VerifyRLSActive performs a runtime check that RLS is enforced on a known table.
// This can be called from health/readiness probes to detect configuration drift.
func VerifyRLSActive(ctx context.Context, pool *pgxpool.Pool) error {
	var rlsEnabled bool
	err := pool.QueryRow(ctx,
		"SELECT relrowsecurity FROM pg_class WHERE relname = 'sessions'",
	).Scan(&rlsEnabled)
	if err != nil {
		return fmt.Errorf("rls verify: cannot check sessions table: %w", err)
	}
	if !rlsEnabled {
		return fmt.Errorf("rls verify: RLS is NOT enabled on sessions table")
	}
	return nil
}
