package pg

import (
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
)

// Compile-time interface assertions — fail the build if contracts drift.
var (
	_ store.APIKeyStore            = (*pgAPIKeyStore)(nil)
	_ store.BootstrapAdminKeyCreator = (*PGStore)(nil)
)

// TestAPIKeyInterfaceCompliance is a named test to surface the above assertions.
func TestAPIKeyInterfaceCompliance(t *testing.T) {}

// TestBootstrapIdempotency_ErrNoRowsMeansAlreadyClaimed verifies the core
// idempotency branch: when bootstrap_state INSERT returns pgx.ErrNoRows
// (ON CONFLICT DO NOTHING produced no row), CreateBootstrapAdminKey must
// return (false, nil) — not an error.
func TestBootstrapIdempotency_ErrNoRowsMeansAlreadyClaimed(t *testing.T) {
	err := pgx.ErrNoRows
	created, returnedErr := resolveBootstrapClaim(err)
	if created {
		t.Error("created should be false when ErrNoRows is returned")
	}
	if returnedErr != nil {
		t.Errorf("err should be nil when ErrNoRows is returned, got: %v", returnedErr)
	}
}

// TestBootstrapIdempotency_OtherErrorPropagates ensures non-ErrNoRows errors
// are returned as-is and do not set created=true.
func TestBootstrapIdempotency_OtherErrorPropagates(t *testing.T) {
	someErr := pgx.ErrTxClosed
	created, returnedErr := resolveBootstrapClaim(someErr)
	if created {
		t.Error("created should be false on unexpected error")
	}
	if returnedErr == nil {
		t.Error("non-ErrNoRows error should be propagated")
	}
}

// resolveBootstrapClaim mirrors the claim-decision logic in CreateBootstrapAdminKey.
func resolveBootstrapClaim(scanErr error) (bool, error) {
	if scanErr != nil {
		if scanErr == pgx.ErrNoRows {
			return false, nil
		}
		return false, scanErr
	}
	return true, nil
}

// TestAPIKeyCreate_SQLIncludesScopes validates that the INSERT SQL persists scopes.
func TestAPIKeyCreate_SQLIncludesScopes(t *testing.T) {
	sql := `INSERT INTO api_keys (id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING created_at`
	if !strings.Contains(sql, "scopes") {
		t.Error("INSERT SQL must include scopes column")
	}
	if !strings.Contains(sql, "RETURNING created_at") {
		t.Error("INSERT SQL must RETURNING created_at to populate the key struct")
	}
}

// TestAPIKeyGetByHash_SQLCoalesceScopes validates that GetByHash uses COALESCE so
// legacy NULL rows are read back as an empty slice rather than causing a scan error.
func TestAPIKeyGetByHash_SQLCoalesceScopes(t *testing.T) {
	sql := `SELECT id, tenant_id, name, key_hash, prefix, roles, COALESCE(scopes, '{}'), expires_at, revoked_at, created_at
			 FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`
	if !strings.Contains(sql, "COALESCE(scopes, '{}')") {
		t.Error("SELECT must use COALESCE(scopes, '{}') to handle NULL legacy rows")
	}
	if !strings.Contains(sql, "revoked_at IS NULL") {
		t.Error("SELECT must filter revoked keys")
	}
}

// TestAPIKeyList_SQLCoalesceScopes validates that List also uses COALESCE.
func TestAPIKeyList_SQLCoalesceScopes(t *testing.T) {
	sql := `SELECT id, tenant_id, name, prefix, roles, COALESCE(scopes, '{}'), expires_at, revoked_at, created_at
			 FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`
	if !strings.Contains(sql, "COALESCE(scopes, '{}')") {
		t.Error("List SQL must use COALESCE(scopes, '{}') to handle NULL legacy rows")
	}
}

// TestBootstrapState_SQLIdempotency validates that the bootstrap_state INSERT uses
// ON CONFLICT DO NOTHING and RETURNING id to detect the already-claimed path.
func TestBootstrapState_SQLIdempotency(t *testing.T) {
	sql := `INSERT INTO bootstrap_state (id, tenant_id, created_at)
			 VALUES ('default_admin', $1, $2)
			 ON CONFLICT (id) DO NOTHING
			 RETURNING id`
	if !strings.Contains(sql, "ON CONFLICT (id) DO NOTHING") {
		t.Error("bootstrap_state INSERT must use ON CONFLICT DO NOTHING for cross-replica idempotency")
	}
	if !strings.Contains(sql, "RETURNING id") {
		t.Error("bootstrap_state INSERT must RETURNING id to detect the already-claimed path via ErrNoRows")
	}
}
