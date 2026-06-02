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

type pgChannelAppStore struct{ pool *pgxpool.Pool }

func (s *pgChannelAppStore) Create(ctx context.Context, app *store.ChannelApp) error {
	if app.ID == "" {
		app.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, app.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO channel_apps
				(id, tenant_id, platform, app_key, app_secret_ref, oauth_secret_ref, webhook_secret_ref, enabled)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at`,
			app.ID, app.TenantID, app.Platform, app.AppKey, nullString(app.AppSecretRef),
			nullString(app.OAuthSecretRef), nullString(app.WebhookSecretRef), app.Enabled,
		).Scan(&app.CreatedAt, &app.UpdatedAt)
	})
}

func (s *pgChannelAppStore) GetByID(ctx context.Context, tenantID, id string) (*store.ChannelApp, error) {
	app := &store.ChannelApp{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at
		FROM channel_apps
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`,
		tenantID, id,
	).Scan(&app.ID, &app.TenantID, &app.Platform, &app.AppKey, &app.AppSecretRef,
		&app.OAuthSecretRef, &app.WebhookSecretRef, &app.Enabled, &app.CreatedAt, &app.UpdatedAt, &app.DeletedAt)
	if err != nil {
		return nil, notFoundOr("get channel app by id", err)
	}
	return app, nil
}

func (s *pgChannelAppStore) GetByPlatformAppKey(ctx context.Context, platform, appKey string) (*store.ChannelApp, error) {
	app := &store.ChannelApp{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at
		FROM channel_apps
		WHERE platform = $1 AND app_key = $2 AND deleted_at IS NULL`,
		platform, appKey,
	).Scan(&app.ID, &app.TenantID, &app.Platform, &app.AppKey, &app.AppSecretRef,
		&app.OAuthSecretRef, &app.WebhookSecretRef, &app.Enabled, &app.CreatedAt, &app.UpdatedAt, &app.DeletedAt)
	if err != nil {
		return nil, notFoundOr("get channel app by platform app key", err)
	}
	return app, nil
}

func (s *pgChannelAppStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.ChannelApp, int, error) {
	limit, offset := normalizeList(opts)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, platform, app_key, COALESCE(app_secret_ref, ''),
		       COALESCE(oauth_secret_ref, ''), COALESCE(webhook_secret_ref, ''),
		       enabled, created_at, updated_at, deleted_at, COUNT(*) OVER()
		FROM channel_apps
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list channel apps: %w", err)
	}
	defer rows.Close()

	var out []*store.ChannelApp
	total := 0
	for rows.Next() {
		app := &store.ChannelApp{}
		if err := rows.Scan(&app.ID, &app.TenantID, &app.Platform, &app.AppKey, &app.AppSecretRef,
			&app.OAuthSecretRef, &app.WebhookSecretRef, &app.Enabled, &app.CreatedAt, &app.UpdatedAt,
			&app.DeletedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scan channel app: %w", err)
		}
		out = append(out, app)
	}
	return out, total, rows.Err()
}

func (s *pgChannelAppStore) Update(ctx context.Context, app *store.ChannelApp) error {
	return withTenantTx(ctx, s.pool, app.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE channel_apps
			SET platform = $3, app_key = $4, app_secret_ref = $5, oauth_secret_ref = $6,
			    webhook_secret_ref = $7, enabled = $8, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
			RETURNING updated_at`,
			app.TenantID, app.ID, app.Platform, app.AppKey, nullString(app.AppSecretRef),
			nullString(app.OAuthSecretRef), nullString(app.WebhookSecretRef), app.Enabled,
		).Scan(&app.UpdatedAt)
		if err != nil {
			return notFoundOr("update channel app", err)
		}
		return nil
	})
}

func (s *pgChannelAppStore) Delete(ctx context.Context, tenantID, id string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE channel_apps SET deleted_at = now(), enabled = false, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`,
			tenantID, id,
		)
		if err != nil {
			return fmt.Errorf("delete channel app: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

type pgChannelIdentityStore struct{ pool *pgxpool.Pool }

func (s *pgChannelIdentityStore) Upsert(ctx context.Context, identity *store.ChannelIdentity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	now := time.Now()
	return withTenantTx(ctx, s.pool, identity.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO channel_identities
				(id, tenant_id, channel_app_id, platform, provider_user_hash, provider_display_name, user_id, last_login_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (channel_app_id, provider_user_hash)
			DO UPDATE SET user_id = EXCLUDED.user_id,
			              provider_display_name = EXCLUDED.provider_display_name,
			              revoked_at = NULL,
			              last_login_at = EXCLUDED.last_login_at
			RETURNING id, created_at, last_login_at, revoked_at`,
			identity.ID, identity.TenantID, identity.ChannelAppID, identity.Platform,
			identity.ProviderUserHash, nullString(identity.ProviderDisplayName), identity.UserID, now,
		).Scan(&identity.ID, &identity.CreatedAt, &identity.LastLoginAt, &identity.RevokedAt)
	})
}

func (s *pgChannelIdentityStore) GetByID(ctx context.Context, tenantID, id string) (*store.ChannelIdentity, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
}

func (s *pgChannelIdentityStore) GetByProviderHash(ctx context.Context, channelAppID, providerUserHash string) (*store.ChannelIdentity, error) {
	return s.getOne(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE channel_app_id = $1 AND provider_user_hash = $2 AND revoked_at IS NULL`,
		channelAppID, providerUserHash)
}

func (s *pgChannelIdentityStore) ListByUser(ctx context.Context, tenantID, userID string) ([]*store.ChannelIdentity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at
		FROM channel_identities
		WHERE tenant_id = $1 AND user_id = $2 AND revoked_at IS NULL
		ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list channel identities by user: %w", err)
	}
	defer rows.Close()
	return scanIdentities(rows)
}

func (s *pgChannelIdentityStore) List(ctx context.Context, tenantID string, opts store.ListOptions) ([]*store.ChannelIdentity, int, error) {
	limit, offset := normalizeList(opts)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, channel_app_id, platform, provider_user_hash, COALESCE(provider_display_name, ''),
		       user_id, created_at, last_login_at, revoked_at, COUNT(*) OVER()
		FROM channel_identities
		WHERE tenant_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channel identities: %w", err)
	}
	defer rows.Close()

	var out []*store.ChannelIdentity
	total := 0
	for rows.Next() {
		identity := &store.ChannelIdentity{}
		if err := scanIdentityWithTotal(rows, identity, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, identity)
	}
	return out, total, rows.Err()
}

func (s *pgChannelIdentityStore) Revoke(ctx context.Context, tenantID, id string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE channel_identities SET revoked_at = now()
			WHERE tenant_id = $1 AND id = $2 AND revoked_at IS NULL`, tenantID, id)
		if err != nil {
			return fmt.Errorf("revoke channel identity: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgChannelIdentityStore) getOne(ctx context.Context, sql string, args ...any) (*store.ChannelIdentity, error) {
	identity := &store.ChannelIdentity{}
	err := s.pool.QueryRow(ctx, sql, args...).Scan(&identity.ID, &identity.TenantID, &identity.ChannelAppID,
		&identity.Platform, &identity.ProviderUserHash, &identity.ProviderDisplayName, &identity.UserID,
		&identity.CreatedAt, &identity.LastLoginAt, &identity.RevokedAt)
	if err != nil {
		return nil, notFoundOr("get channel identity", err)
	}
	return identity, nil
}

type pgBrowserSessionStore struct{ pool *pgxpool.Pool }

func (s *pgBrowserSessionStore) Create(ctx context.Context, session *store.BrowserSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, session.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO browser_sessions
				(id, tenant_id, user_id, token_hash, csrf_token_hash, user_agent, source_ip, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, last_seen_at, revoked_at`,
			session.ID, session.TenantID, session.UserID, session.TokenHash, session.CSRFTokenHash,
			nullString(session.UserAgent), nullString(session.SourceIP), session.ExpiresAt,
		).Scan(&session.CreatedAt, &session.LastSeenAt, &session.RevokedAt)
	})
}

func (s *pgBrowserSessionStore) GetByHash(ctx context.Context, tokenHash string) (*store.BrowserSession, error) {
	session := &store.BrowserSession{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, token_hash, csrf_token_hash, COALESCE(user_agent, ''),
		       COALESCE(source_ip, ''), created_at, last_seen_at, expires_at, revoked_at
		FROM browser_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`,
		tokenHash,
	).Scan(&session.ID, &session.TenantID, &session.UserID, &session.TokenHash, &session.CSRFTokenHash,
		&session.UserAgent, &session.SourceIP, &session.CreatedAt, &session.LastSeenAt,
		&session.ExpiresAt, &session.RevokedAt)
	if err != nil {
		return nil, notFoundOr("get browser session by hash", err)
	}
	return session, nil
}

func (s *pgBrowserSessionStore) Touch(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE browser_sessions SET last_seen_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("touch browser session: %w", err)
	}
	return nil
}

func (s *pgBrowserSessionStore) Revoke(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE browser_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke browser session: %w", err)
	}
	return nil
}

func (s *pgBrowserSessionStore) RevokeByUser(ctx context.Context, tenantID, userID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE browser_sessions SET revoked_at = now()
			WHERE tenant_id = $1 AND user_id = $2 AND revoked_at IS NULL`, tenantID, userID)
		if err != nil {
			return fmt.Errorf("revoke browser sessions by user: %w", err)
		}
		return nil
	})
}

func scanIdentities(rows pgx.Rows) ([]*store.ChannelIdentity, error) {
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

func scanIdentityWithTotal(rows pgx.Rows, identity *store.ChannelIdentity, total *int) error {
	if err := rows.Scan(&identity.ID, &identity.TenantID, &identity.ChannelAppID, &identity.Platform,
		&identity.ProviderUserHash, &identity.ProviderDisplayName, &identity.UserID,
		&identity.CreatedAt, &identity.LastLoginAt, &identity.RevokedAt, total); err != nil {
		return fmt.Errorf("scan channel identity: %w", err)
	}
	return nil
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
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return fmt.Errorf("%s: %w", label, err)
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

var _ store.ChannelAppStore = (*pgChannelAppStore)(nil)
var _ store.ChannelIdentityStore = (*pgChannelIdentityStore)(nil)
var _ store.BrowserSessionStore = (*pgBrowserSessionStore)(nil)
