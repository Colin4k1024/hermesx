package mysql

import (
	"context"
	"database/sql"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myUserProfileStore struct{ db *sql.DB }

func (s *myUserProfileStore) Get(ctx context.Context, tenantID, userID string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM user_profiles WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func (s *myUserProfileStore) Upsert(ctx context.Context, tenantID, userID, content string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_profiles (tenant_id, user_id, content)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE content = VALUES(content)`,
		tenantID, userID, content)
	return err
}

func (s *myUserProfileStore) Delete(ctx context.Context, tenantID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM user_profiles WHERE tenant_id = ? AND user_id = ?`, tenantID, userID)
	return err
}

func (s *myUserProfileStore) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM user_profiles WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var _ store.UserProfileStore = (*myUserProfileStore)(nil)
