package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myUserStore struct{ db *sql.DB }

func (u *myUserStore) GetOrCreate(ctx context.Context, tenantID, externalID, username string) (*store.User, error) {
	user := &store.User{}
	err := u.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, external_id, COALESCE(username,''), COALESCE(display_name,''), role, approved_at
		 FROM users WHERE tenant_id = ? AND external_id = ?`, tenantID, externalID).
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
	_, err = u.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, external_id, username, role) VALUES (?, ?, ?, ?, ?)`,
		user.ID, tenantID, externalID, username, user.Role)
	return user, err
}

func (u *myUserStore) IsApproved(ctx context.Context, tenantID, platform, userID string) (bool, error) {
	externalID := platform + ":" + userID
	var approvedAt *time.Time
	err := u.db.QueryRowContext(ctx,
		`SELECT approved_at FROM users WHERE tenant_id = ? AND external_id = ?`,
		tenantID, externalID).Scan(&approvedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check user approved: %w", err)
	}
	return approvedAt != nil, nil
}

func (u *myUserStore) Approve(ctx context.Context, tenantID, platform, userID string) error {
	externalID := platform + ":" + userID
	_, err := u.db.ExecContext(ctx,
		`UPDATE users SET approved_at = ? WHERE tenant_id = ? AND external_id = ?`,
		time.Now(), tenantID, externalID)
	return err
}

func (u *myUserStore) Revoke(ctx context.Context, tenantID, platform, userID string) error {
	externalID := platform + ":" + userID
	_, err := u.db.ExecContext(ctx,
		`UPDATE users SET approved_at = NULL WHERE tenant_id = ? AND external_id = ?`,
		tenantID, externalID)
	return err
}

func (u *myUserStore) ListApproved(ctx context.Context, tenantID, platform string) ([]string, error) {
	prefix := platform + ":"
	rows, err := u.db.QueryContext(ctx,
		`SELECT external_id FROM users WHERE tenant_id = ? AND external_id LIKE ? AND approved_at IS NOT NULL`,
		tenantID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var eid string
		if err := rows.Scan(&eid); err != nil {
			return nil, err
		}
		ids = append(ids, eid[len(prefix):])
	}
	return ids, rows.Err()
}

// CreateWithPassword inserts a new user with a bcrypt password hash (self-registration).
func (u *myUserStore) CreateWithPassword(ctx context.Context, user *store.User, passwordHash string) error {
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
	user.ApprovedAt = &now

	_, err := u.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, external_id, username, display_name, role, approved_at, password_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.TenantID, user.ExternalID, user.Username, user.DisplayName, user.Role, user.ApprovedAt, passwordHash)
	if err != nil {
		return fmt.Errorf("create user with password: %w", err)
	}
	return nil
}

// GetByUsername looks up a user by tenant-scoped username and returns both the user and the password hash.
func (u *myUserStore) GetByUsername(ctx context.Context, tenantID, username string) (*store.User, string, error) {
	user := &store.User{}
	var passwordHash string
	err := u.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, external_id, username, display_name, role, approved_at, password_hash
		 FROM users WHERE tenant_id = ? AND username = ?`, tenantID, username).
		Scan(&user.ID, &user.TenantID, &user.ExternalID, &user.Username, &user.DisplayName, &user.Role, &user.ApprovedAt, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", store.ErrNotFound
		}
		return nil, "", fmt.Errorf("get user by username: %w", err)
	}
	return user, passwordHash, nil
}

// GetByID looks up a user by tenant-scoped user ID.
func (u *myUserStore) GetByID(ctx context.Context, tenantID, userID string) (*store.User, error) {
	user := &store.User{}
	err := u.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, external_id, username, display_name, role, approved_at
		 FROM users WHERE tenant_id = ? AND id = ?`, tenantID, userID).
		Scan(&user.ID, &user.TenantID, &user.ExternalID, &user.Username, &user.DisplayName, &user.Role, &user.ApprovedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

var _ store.UserStore = (*myUserStore)(nil)
