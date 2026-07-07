package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgFileEntryStore struct {
	pool *pgxpool.Pool
}

func (s *pgFileEntryStore) List(ctx context.Context, tenantID, userID string) ([]*store.FileEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, path, minio_key, size_bytes, mime_type, sha256,
		        source_session, created_at, updated_at
		 FROM file_entries
		 WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("pg list file entries: %w", err)
	}
	defer rows.Close()

	var entries []*store.FileEntry
	for rows.Next() {
		var e store.FileEntry
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.UserID, &e.Path, &e.MinIOKey,
			&e.SizeBytes, &e.MIMEType, &e.SHA256, &e.SourceSession,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("pg scan file entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *pgFileEntryStore) Get(ctx context.Context, tenantID, userID, id string) (*store.FileEntry, error) {
	var e store.FileEntry
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, path, minio_key, size_bytes, mime_type, sha256,
		        source_session, created_at, updated_at
		 FROM file_entries
		 WHERE tenant_id = $1 AND user_id = $2 AND id = $3 AND deleted_at IS NULL`,
		tenantID, userID, id).Scan(
		&e.ID, &e.TenantID, &e.UserID, &e.Path, &e.MinIOKey,
		&e.SizeBytes, &e.MIMEType, &e.SHA256, &e.SourceSession,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("pg get file entry: %w", err)
	}
	return &e, nil
}

func (s *pgFileEntryStore) Create(ctx context.Context, entry *store.FileEntry) error {
	return withTenantTx(ctx, s.pool, entry.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO file_entries (id, tenant_id, user_id, path, minio_key, size_bytes, mime_type, sha256, source_session, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			entry.ID, entry.TenantID, entry.UserID, entry.Path, entry.MinIOKey,
			entry.SizeBytes, entry.MIMEType, entry.SHA256, entry.SourceSession,
			entry.CreatedAt, entry.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("pg create file entry: %w", err)
		}
		return nil
	})
}

func (s *pgFileEntryStore) Delete(ctx context.Context, tenantID, userID, id string) error {
	var affected int64
	err := withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE file_entries SET deleted_at = now()
			 WHERE tenant_id = $1 AND user_id = $2 AND id = $3 AND deleted_at IS NULL`,
			tenantID, userID, id)
		if err != nil {
			return fmt.Errorf("pg delete file entry: %w", err)
		}
		affected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *pgFileEntryStore) GetUserStorageUsage(ctx context.Context, tenantID, userID string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(size_bytes), 0)
		 FROM file_entries
		 WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		tenantID, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("pg get user storage usage: %w", err)
	}
	return total, nil
}

var _ store.FileEntryStore = (*pgFileEntryStore)(nil)
