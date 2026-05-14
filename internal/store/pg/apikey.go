package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAPIKeyStore struct {
	pool *pgxpool.Pool
}

func (s *pgAPIKeyStore) Create(ctx context.Context, key *store.APIKey) error {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, key.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`INSERT INTO api_keys (id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING created_at`,
			key.ID, key.TenantID, key.Name, key.KeyHash, key.Prefix, key.Roles, key.Scopes, key.ExpiresAt,
		).Scan(&key.CreatedAt)
		if err != nil {
			return fmt.Errorf("create api key: %w", err)
		}
		return nil
	})
}

func (s *pgAPIKeyStore) GetByHash(ctx context.Context, hash string) (*store.APIKey, error) {
	k := &store.APIKey{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, key_hash, prefix, roles, COALESCE(scopes, '{}'), expires_at, revoked_at, created_at
		 FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`,
		hash,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.KeyHash, &k.Prefix, &k.Roles, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return k, nil
}

func (s *pgAPIKeyStore) GetByID(ctx context.Context, tenantID, id string) (*store.APIKey, error) {
	k := &store.APIKey{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, key_hash, prefix, roles, COALESCE(scopes, '{}'), expires_at, revoked_at, created_at
		 FROM api_keys WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.KeyHash, &k.Prefix, &k.Roles, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	return k, nil
}

func (s *pgAPIKeyStore) List(ctx context.Context, tenantID string) ([]*store.APIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, prefix, roles, COALESCE(scopes, '{}'), expires_at, revoked_at, created_at
		 FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*store.APIKey
	for rows.Next() {
		k := &store.APIKey{}
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Roles, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

func (s *pgAPIKeyStore) Revoke(ctx context.Context, tenantID, id string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE api_keys SET revoked_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id)
		if err != nil {
			return fmt.Errorf("revoke api key: %w", err)
		}
		return nil
	})
}

var _ store.APIKeyStore = (*pgAPIKeyStore)(nil)

// CreateBootstrapAdminKey atomically claims the platform bootstrap slot and
// creates the first admin key. It returns created=false when another replica
// already completed bootstrap.
func (s *PGStore) CreateBootstrapAdminKey(ctx context.Context, key *store.APIKey) (bool, error) {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	tx, err := beginTenantTx(ctx, s.pool, key.TenantID)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var claimed string
	err = tx.QueryRow(ctx,
		`INSERT INTO bootstrap_state (id, tenant_id, created_at)
		 VALUES ('default_admin', $1, $2)
		 ON CONFLICT (id) DO NOTHING
		 RETURNING id`,
		key.TenantID, time.Now(),
	).Scan(&claimed)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("claim bootstrap: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO api_keys (id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING created_at`,
		key.ID, key.TenantID, key.Name, key.KeyHash, key.Prefix, key.Roles, key.Scopes, key.ExpiresAt,
	).Scan(&key.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("create bootstrap api key: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE bootstrap_state SET key_id = $1 WHERE id = 'default_admin'`, key.ID); err != nil {
		return false, fmt.Errorf("record bootstrap key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit bootstrap: %w", err)
	}
	return true, nil
}

var _ store.BootstrapAdminKeyCreator = (*PGStore)(nil)
