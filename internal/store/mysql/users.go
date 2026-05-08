package mysql

import (
	"context"
	"database/sql"
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
		return false, nil
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

var _ store.UserStore = (*myUserStore)(nil)
