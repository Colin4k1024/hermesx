package pg

import "github.com/jackc/pgx/v5/pgxpool"

// PoolProvider is implemented by PGStore and allows callers to extract the
// underlying pgxpool for transactional operations.
// This is deliberately kept in the pg package so that other store backends
// (e.g. MySQL) do not have a compile-time pgxpool dependency.
type PoolProvider interface {
	Pool() *pgxpool.Pool
}
