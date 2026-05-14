package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myAPIKeyStore struct{ db *sql.DB }

func (s *myAPIKeyStore) Create(ctx context.Context, key *store.APIKey) error {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	roles, _ := json.Marshal(key.Roles)
	scopes, _ := json.Marshal(key.Scopes)
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.TenantID, key.Name, key.KeyHash, key.Prefix, string(roles), string(scopes), key.ExpiresAt, key.CreatedAt)
	return err
}

func (s *myAPIKeyStore) GetByHash(ctx context.Context, hash string) (*store.APIKey, error) {
	k, err := s.scanKey(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at, revoked_at, created_at
		 FROM api_keys WHERE key_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`, hash))
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return k, nil
}

func (s *myAPIKeyStore) GetByID(ctx context.Context, tenantID, id string) (*store.APIKey, error) {
	k, err := s.scanKey(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at, revoked_at, created_at
		 FROM api_keys WHERE tenant_id = ? AND id = ?`, tenantID, id))
	if err != nil {
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	return k, nil
}

func (s *myAPIKeyStore) List(ctx context.Context, tenantID string) ([]*store.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at, revoked_at, created_at
		 FROM api_keys WHERE tenant_id = ? ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*store.APIKey
	for rows.Next() {
		k := &store.APIKey{}
		var rolesJSON, scopesJSON string
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.KeyHash, &k.Prefix,
			&rolesJSON, &scopesJSON, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if err := json.Unmarshal([]byte(rolesJSON), &k.Roles); err != nil {
			return nil, fmt.Errorf("unmarshal roles: %w", err)
		}
		if err := json.Unmarshal([]byte(scopesJSON), &k.Scopes); err != nil {
			return nil, fmt.Errorf("unmarshal scopes: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *myAPIKeyStore) Revoke(ctx context.Context, tenantID, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}

func (s *myAPIKeyStore) scanKey(row *sql.Row) (*store.APIKey, error) {
	k := &store.APIKey{}
	var rolesJSON, scopesJSON string
	if err := row.Scan(&k.ID, &k.TenantID, &k.Name, &k.KeyHash, &k.Prefix,
		&rolesJSON, &scopesJSON, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rolesJSON), &k.Roles); err != nil {
		return nil, fmt.Errorf("unmarshal roles: %w", err)
	}
	if err := json.Unmarshal([]byte(scopesJSON), &k.Scopes); err != nil {
		return nil, fmt.Errorf("unmarshal scopes: %w", err)
	}
	return k, nil
}

var _ store.APIKeyStore = (*myAPIKeyStore)(nil)

// CreateBootstrapAdminKey atomically claims the platform bootstrap slot and
// creates the first admin key. It returns created=false when another replica
// already completed bootstrap.
func (s *MySQLStore) CreateBootstrapAdminKey(ctx context.Context, key *store.APIKey) (bool, error) {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}
	roles, _ := json.Marshal(key.Roles)
	scopes, _ := json.Marshal(key.Scopes)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin bootstrap tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO bootstrap_state (id, tenant_id, created_at) VALUES ('default_admin', ?, ?)`,
		key.TenantID, key.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("claim bootstrap: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("bootstrap claim result: %w", err)
	}
	if rows == 0 {
		return false, nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO api_keys (id, tenant_id, name, key_hash, prefix, roles, scopes, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.TenantID, key.Name, key.KeyHash, key.Prefix, string(roles), string(scopes), key.ExpiresAt, key.CreatedAt,
	); err != nil {
		return false, fmt.Errorf("create bootstrap api key: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE bootstrap_state SET key_id = ? WHERE id = 'default_admin'`,
		key.ID,
	); err != nil {
		return false, fmt.Errorf("record bootstrap key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit bootstrap: %w", err)
	}
	return true, nil
}

var _ store.BootstrapAdminKeyCreator = (*MySQLStore)(nil)
