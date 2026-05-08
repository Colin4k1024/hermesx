package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myMemoryStore struct{ db *sql.DB }

func (s *myMemoryStore) Get(ctx context.Context, tenantID, userID, key string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM memories WHERE tenant_id = ? AND user_id = ? AND key_name = ?`,
		tenantID, userID, key).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func (s *myMemoryStore) List(ctx context.Context, tenantID, userID string) ([]store.MemoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tenant_id, user_id, key_name, content, updated_at FROM memories
		 WHERE tenant_id = ? AND user_id = ? ORDER BY updated_at DESC`,
		tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []store.MemoryEntry
	for rows.Next() {
		var e store.MemoryEntry
		if err := rows.Scan(&e.TenantID, &e.UserID, &e.Key, &e.Content, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *myMemoryStore) Upsert(ctx context.Context, tenantID, userID, key, content string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memories (tenant_id, user_id, key_name, content)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE content = VALUES(content)`,
		tenantID, userID, key, content)
	return err
}

func (s *myMemoryStore) Delete(ctx context.Context, tenantID, userID, key string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM memories WHERE tenant_id = ? AND user_id = ? AND key_name = ?`,
		tenantID, userID, key)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory key %q not found", key)
	}
	return nil
}

func (s *myMemoryStore) DeleteAllByUser(ctx context.Context, tenantID, userID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM memories WHERE tenant_id = ? AND user_id = ?`, tenantID, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *myMemoryStore) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM memories WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var _ store.MemoryStore = (*myMemoryStore)(nil)
