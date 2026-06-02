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

type myChannelAppStore struct{ db *sql.DB }

func (s *myChannelAppStore) Create(ctx context.Context, app *store.ChannelApp) error {
	if app.ID == "" {
		app.ID = uuid.New().String()
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_apps
			(id, tenant_id, platform, app_key, app_secret_ref, oauth_secret_ref, webhook_secret_ref, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.TenantID, app.Platform, app.AppKey, nullStr(app.AppSecretRef),
		nullStr(app.OAuthSecretRef), nullStr(app.WebhookSecretRef), app.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("create channel app: %w", err)
	}
	app.CreatedAt = now
	app.UpdatedAt = now
	return nil
}

func (s *myChannelAppStore) GetByID(ctx context.Context, tenantID, id string) (*store.ChannelApp, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at
		FROM channel_apps
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, tenantID, id)
}

func (s *myChannelAppStore) GetByPlatformAppKey(ctx context.Context, platform, appKey string) (*store.ChannelApp, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at
		FROM channel_apps
		WHERE platform = ? AND app_key = ? AND deleted_at IS NULL`, platform, appKey)
}

func (s *myChannelAppStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.ChannelApp, int, error) {
	limit, offset := normalizeList(opts)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at
		FROM channel_apps
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channel apps: %w", err)
	}
	defer rows.Close()

	var out []*store.ChannelApp
	for rows.Next() {
		app := &store.ChannelApp{}
		if err := rows.Scan(&app.ID, &app.TenantID, &app.Platform, &app.AppKey, &app.AppSecretRef,
			&app.OAuthSecretRef, &app.WebhookSecretRef, &app.Enabled, &app.CreatedAt, &app.UpdatedAt,
			&app.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan channel app: %w", err)
		}
		out = append(out, app)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, len(out), nil
}

func (s *myChannelAppStore) Update(ctx context.Context, app *store.ChannelApp) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE channel_apps
		SET platform = ?, app_key = ?, app_secret_ref = ?, oauth_secret_ref = ?,
		    webhook_secret_ref = ?, enabled = ?
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`,
		app.Platform, app.AppKey, nullStr(app.AppSecretRef), nullStr(app.OAuthSecretRef),
		nullStr(app.WebhookSecretRef), app.Enabled, app.TenantID, app.ID)
	if err != nil {
		return fmt.Errorf("update channel app: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *myChannelAppStore) Delete(ctx context.Context, tenantID, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE channel_apps SET deleted_at = CURRENT_TIMESTAMP(3), enabled = 0
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete channel app: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *myChannelAppStore) getOne(ctx context.Context, query string, args ...any) (*store.ChannelApp, error) {
	app := &store.ChannelApp{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&app.ID, &app.TenantID, &app.Platform, &app.AppKey,
		&app.AppSecretRef, &app.OAuthSecretRef, &app.WebhookSecretRef, &app.Enabled,
		&app.CreatedAt, &app.UpdatedAt, &app.DeletedAt)
	if err != nil {
		return nil, notFoundOr("get channel app", err)
	}
	return app, nil
}

type myChannelIdentityStore struct{ db *sql.DB }

func (s *myChannelIdentityStore) Upsert(ctx context.Context, identity *store.ChannelIdentity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_identities
			(id, tenant_id, channel_app_id, platform, provider_user_hash, provider_display_name, user_id, last_login_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			provider_display_name = VALUES(provider_display_name),
			revoked_at = NULL,
			last_login_at = VALUES(last_login_at)`,
		identity.ID, identity.TenantID, identity.ChannelAppID, identity.Platform,
		identity.ProviderUserHash, nullStr(identity.ProviderDisplayName), identity.UserID, now,
	)
	if err != nil {
		return fmt.Errorf("upsert channel identity: %w", err)
	}
	stored, err := s.GetByProviderHash(ctx, identity.ChannelAppID, identity.ProviderUserHash)
	if err != nil {
		return err
	}
	*identity = *stored
	return nil
}

func (s *myChannelIdentityStore) GetByID(ctx context.Context, tenantID, id string) (*store.ChannelIdentity, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE tenant_id = ? AND id = ?`, tenantID, id)
}

func (s *myChannelIdentityStore) GetByProviderHash(ctx context.Context, channelAppID, providerUserHash string) (*store.ChannelIdentity, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE channel_app_id = ? AND provider_user_hash = ? AND revoked_at IS NULL`,
		channelAppID, providerUserHash)
}

func (s *myChannelIdentityStore) ListByUser(ctx context.Context, tenantID, userID string) ([]*store.ChannelIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE tenant_id = ? AND user_id = ? AND revoked_at IS NULL
		ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list channel identities by user: %w", err)
	}
	defer rows.Close()
	return scanIdentities(rows)
}

func (s *myChannelIdentityStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.ChannelIdentity, int, error) {
	limit, offset := normalizeList(opts)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE tenant_id = ? AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channel identities: %w", err)
	}
	defer rows.Close()
	out, err := scanIdentities(rows)
	return out, len(out), err
}

func (s *myChannelIdentityStore) Revoke(ctx context.Context, tenantID, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE channel_identities SET revoked_at = CURRENT_TIMESTAMP(3)
		WHERE tenant_id = ? AND id = ? AND revoked_at IS NULL`, tenantID, id)
	if err != nil {
		return fmt.Errorf("revoke channel identity: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *myChannelIdentityStore) getOne(ctx context.Context, query string, args ...any) (*store.ChannelIdentity, error) {
	identity := &store.ChannelIdentity{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&identity.ID, &identity.TenantID,
		&identity.ChannelAppID, &identity.Platform, &identity.ProviderUserHash, &identity.ProviderDisplayName,
		&identity.UserID, &identity.CreatedAt, &identity.LastLoginAt, &identity.RevokedAt)
	if err != nil {
		return nil, notFoundOr("get channel identity", err)
	}
	return identity, nil
}

type myBrowserSessionStore struct{ db *sql.DB }

func (s *myBrowserSessionStore) Create(ctx context.Context, session *store.BrowserSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO browser_sessions
			(id, tenant_id, user_id, token_hash, csrf_token_hash, user_agent, source_ip, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.TenantID, session.UserID, session.TokenHash, session.CSRFTokenHash,
		nullStr(session.UserAgent), nullStr(session.SourceIP), session.ExpiresAt, now,
	)
	if err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	session.CreatedAt = now
	return nil
}

func (s *myBrowserSessionStore) GetByHash(ctx context.Context, tokenHash string) (*store.BrowserSession, error) {
	session := &store.BrowserSession{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, token_hash, csrf_token_hash, COALESCE(user_agent, ''),
		       COALESCE(source_ip, ''), created_at, last_seen_at, expires_at, revoked_at
		FROM browser_sessions
		WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP(3)`,
		tokenHash,
	).Scan(&session.ID, &session.TenantID, &session.UserID, &session.TokenHash, &session.CSRFTokenHash,
		&session.UserAgent, &session.SourceIP, &session.CreatedAt, &session.LastSeenAt,
		&session.ExpiresAt, &session.RevokedAt)
	if err != nil {
		return nil, notFoundOr("get browser session by hash", err)
	}
	return session, nil
}

func (s *myBrowserSessionStore) Touch(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE browser_sessions SET last_seen_at = CURRENT_TIMESTAMP(3) WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("touch browser session: %w", err)
	}
	return nil
}

func (s *myBrowserSessionStore) Revoke(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at = CURRENT_TIMESTAMP(3) WHERE id = ? AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke browser session: %w", err)
	}
	return nil
}

func (s *myBrowserSessionStore) RevokeByUser(ctx context.Context, tenantID, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE browser_sessions SET revoked_at = CURRENT_TIMESTAMP(3)
		WHERE tenant_id = ? AND user_id = ? AND revoked_at IS NULL`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("revoke browser sessions by user: %w", err)
	}
	return nil
}

func scanIdentities(rows *sql.Rows) ([]*store.ChannelIdentity, error) {
	var out []*store.ChannelIdentity
	for rows.Next() {
		identity := &store.ChannelIdentity{}
		if err := rows.Scan(&identity.ID, &identity.TenantID, &identity.ChannelAppID, &identity.Platform,
			&identity.ProviderUserHash, &identity.ProviderDisplayName, &identity.UserID,
			&identity.CreatedAt, &identity.LastLoginAt, &identity.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan channel identity: %w", err)
		}
		out = append(out, identity)
	}
	return out, rows.Err()
}

func normalizeList(opts store.ListOptions) (limit, offset int) {
	limit = opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if opts.Offset > 0 {
		offset = opts.Offset
	}
	return limit, offset
}

func notFoundOr(label string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	return fmt.Errorf("%s: %w", label, err)
}

var _ store.ChannelAppStore = (*myChannelAppStore)(nil)
var _ store.ChannelIdentityStore = (*myChannelIdentityStore)(nil)
var _ store.BrowserSessionStore = (*myBrowserSessionStore)(nil)
