package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgUserStore struct{ pool *pgxpool.Pool }

func (u *pgUserStore) GetOrCreate(ctx context.Context, tenantID, externalID, username string) (*store.User, error) {
	user := &store.User{}
	err := u.pool.QueryRow(ctx,
		`SELECT id, tenant_id, external_id, username, display_name, role, approved_at
		 FROM users WHERE tenant_id = $1 AND external_id = $2`, tenantID, externalID).
		Scan(&user.ID, &user.TenantID, &user.ExternalID, &user.Username, &user.DisplayName, &user.Role, &user.ApprovedAt)

	if err == nil {
		return user, nil
	}

	user = &store.User{
		ID:         uuid.New().String(),
		TenantID:   tenantID,
		ExternalID: externalID,
		Username:   username,
		Role:       "user",
	}
	err = withTenantTx(ctx, u.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO users (id, tenant_id, external_id, username, role) VALUES ($1, $2, $3, $4, $5)`,
			user.ID, tenantID, externalID, username, user.Role)
		return err
	})
	return user, err
}

func (u *pgUserStore) IsApproved(ctx context.Context, tenantID, platform, userID string) (bool, error) {
	externalID := platform + ":" + userID
	var approvedAt *time.Time
	err := u.pool.QueryRow(ctx,
		`SELECT approved_at FROM users WHERE tenant_id = $1 AND external_id = $2`,
		tenantID, externalID).Scan(&approvedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check user approved: %w", err)
	}
	return approvedAt != nil, nil
}

func (u *pgUserStore) Approve(ctx context.Context, tenantID, platform, userID string) error {
	externalID := platform + ":" + userID
	return withTenantTx(ctx, u.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE users SET approved_at = $1 WHERE tenant_id = $2 AND external_id = $3`,
			time.Now(), tenantID, externalID)
		return err
	})
}

func (u *pgUserStore) Revoke(ctx context.Context, tenantID, platform, userID string) error {
	externalID := platform + ":" + userID
	return withTenantTx(ctx, u.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE users SET approved_at = NULL WHERE tenant_id = $1 AND external_id = $2`,
			tenantID, externalID)
		return err
	})
}

func (u *pgUserStore) ListApproved(ctx context.Context, tenantID, platform string) ([]string, error) {
	prefix := platform + ":"
	rows, err := u.pool.Query(ctx,
		`SELECT external_id FROM users WHERE tenant_id = $1 AND external_id LIKE $2 AND approved_at IS NOT NULL`,
		tenantID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var eid string
		if err := rows.Scan(&eid); err != nil {
			return nil, fmt.Errorf("scan approved user: %w", err)
		}
		ids = append(ids, eid[len(prefix):])
	}
	return ids, rows.Err()
}

// CreateWithPassword inserts a new user with a bcrypt password hash (self-registration).
// The external_id is set to "local:<username>" to distinguish from channel/API key users.
func (u *pgUserStore) CreateWithPassword(ctx context.Context, user *store.User, passwordHash string) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.ExternalID == "" {
		user.ExternalID = "local:" + user.Username
	}
	if user.Role == "" {
		user.Role = "user"
	}
	now := time.Now()
	user.ApprovedAt = &now // auto-approve self-registered users

	return withTenantTx(ctx, u.pool, user.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO users (id, tenant_id, external_id, username, display_name, role, approved_at, password_hash)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			user.ID, user.TenantID, user.ExternalID, user.Username, user.DisplayName, user.Role, user.ApprovedAt, passwordHash)
		if err != nil {
			return fmt.Errorf("create user with password: %w", err)
		}
		return nil
	})
}

// GetByUsername looks up a user by tenant-scoped username and returns both the user and the password hash.
func (u *pgUserStore) GetByUsername(ctx context.Context, tenantID, username string) (*store.User, string, error) {
	user := &store.User{}
	var passwordHash string
	err := u.pool.QueryRow(ctx,
		`SELECT id, tenant_id, external_id, username, display_name, role, approved_at, password_hash
		 FROM users WHERE tenant_id = $1 AND username = $2`, tenantID, username).
		Scan(&user.ID, &user.TenantID, &user.ExternalID, &user.Username, &user.DisplayName, &user.Role, &user.ApprovedAt, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", store.ErrNotFound
		}
		return nil, "", fmt.Errorf("get user by username: %w", err)
	}
	return user, passwordHash, nil
}

// GetByID looks up a user by tenant-scoped user ID.
func (u *pgUserStore) GetByID(ctx context.Context, tenantID, userID string) (*store.User, error) {
	user := &store.User{}
	err := u.pool.QueryRow(ctx,
		`SELECT id, tenant_id, external_id, username, display_name, role, approved_at
		 FROM users WHERE tenant_id = $1 AND id = $2`, tenantID, userID).
		Scan(&user.ID, &user.TenantID, &user.ExternalID, &user.Username, &user.DisplayName, &user.Role, &user.ApprovedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}
