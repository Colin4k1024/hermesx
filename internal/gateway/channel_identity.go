package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/channel"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type GatewayIdentityStatus string

const (
	GatewayIdentitySkipped  GatewayIdentityStatus = "skipped"
	GatewayIdentityBound    GatewayIdentityStatus = "bound"
	GatewayIdentityUnbound  GatewayIdentityStatus = "unbound"
	GatewayIdentityRejected GatewayIdentityStatus = "rejected"
)

type GatewayIdentityResult struct {
	Status  GatewayIdentityStatus
	Message string
}

type GatewayIdentityResolverConfig struct {
	Apps       store.ChannelAppStore
	Identities store.ChannelIdentityStore
	AuditLogs  store.AuditLogStore
	Challenges *channel.ChallengeStore
	HashSecret string
	PublicURL  string
	ReturnTo   string
}

// GatewayIdentityResolver maps trusted channel principals to tenant-scoped
// SaaS users. It never accepts tenant_id from message payloads.
type GatewayIdentityResolver struct {
	apps       store.ChannelAppStore
	identities store.ChannelIdentityStore
	auditLogs  store.AuditLogStore
	challenges *channel.ChallengeStore
	hashSecret string
	publicURL  string
	returnTo   string
}

func NewGatewayIdentityResolver(cfg GatewayIdentityResolverConfig) *GatewayIdentityResolver {
	return &GatewayIdentityResolver{
		apps:       cfg.Apps,
		identities: cfg.Identities,
		auditLogs:  cfg.AuditLogs,
		challenges: cfg.Challenges,
		hashSecret: cfg.HashSecret,
		publicURL:  strings.TrimRight(cfg.PublicURL, "/"),
		returnTo:   defaultString(cfg.ReturnTo, "/"),
	}
}

func (r *GatewayIdentityResolver) Resolve(ctx context.Context, event *MessageEvent) GatewayIdentityResult {
	if r == nil || event == nil || !isTrustedChannelPlatform(event.Source.Platform) {
		return GatewayIdentityResult{Status: GatewayIdentitySkipped}
	}
	if r.apps == nil || r.identities == nil || r.challenges == nil || r.hashSecret == "" {
		return GatewayIdentityResult{Status: GatewayIdentitySkipped}
	}
	appKey := strings.TrimSpace(event.Metadata["app_key"])
	if appKey == "" {
		return GatewayIdentityResult{Status: GatewayIdentitySkipped}
	}
	providerUserID := strings.TrimSpace(event.Source.UserID)
	if providerUserID == "" {
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "Unable to identify the channel user. Please contact the administrator to check the channel callback configuration.",
		}
	}
	platform := string(event.Source.Platform)
	app, err := r.apps.GetByPlatformAppKey(ctx, platform, appKey)
	if err != nil || app == nil || !app.Enabled {
		r.audit(ctx, "", "", "CHANNEL_AUTH_FAILED", "gateway channel app disabled or not found: platform="+platform)
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "The channel application is not enabled or not bound to a tenant. Please contact the administrator to configure the HermesX channel application.",
		}
	}
	providerHash, err := channel.HashProviderUser(r.hashSecret, platform, app.AppKey, providerUserID)
	if err != nil {
		r.audit(ctx, app.TenantID, "", "CHANNEL_AUTH_FAILED", "gateway provider hash failed: "+err.Error())
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "Channel login configuration is incomplete. Please contact the administrator.",
		}
	}
	identity, err := r.identities.GetByProviderHash(ctx, app.ID, providerHash)
	if err == nil && identity != nil {
		event.Source.TenantID = app.TenantID
		event.Source.UserID = identity.UserID
		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}
		event.Metadata["provider_user_hash"] = providerHash
		return GatewayIdentityResult{Status: GatewayIdentityBound}
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		r.audit(ctx, app.TenantID, "", "CHANNEL_AUTH_FAILED", "gateway binding lookup failed: "+err.Error())
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "Failed to query binding status. Please try again later.",
		}
	}

	challenge, err := r.challenges.Create(platform, app.AppKey, providerHash, r.returnTo)
	if err != nil {
		r.audit(ctx, app.TenantID, "", "CHANNEL_AUTH_FAILED", "gateway challenge failed: "+err.Error())
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "Failed to generate login link. Please try again later.",
		}
	}
	loginURL, err := r.loginURL(platform, app.AppKey, challenge.ID)
	if err != nil {
		r.audit(ctx, app.TenantID, "", "CHANNEL_AUTH_FAILED", "gateway login url failed: "+err.Error())
		return GatewayIdentityResult{
			Status:  GatewayIdentityRejected,
			Message: "HermesX login endpoint is not configured. Please contact the administrator to configure SAAS_PUBLIC_URL.",
		}
	}
	r.audit(ctx, app.TenantID, "", "GATEWAY_UNBOUND_MESSAGE", "platform="+platform+",app_id="+app.ID)
	return GatewayIdentityResult{
		Status: GatewayIdentityUnbound,
		Message: "Please complete HermesX login and channel binding first:\n" + loginURL +
			"\n\nThis link is valid for a limited time. Once completed, messages sent via this channel will automatically route to the corresponding SaaS tenant.",
	}
}

func (r *GatewayIdentityResolver) loginURL(platform, appKey, challengeID string) (string, error) {
	if r.publicURL == "" {
		return "", fmt.Errorf("public url is required")
	}
	q := url.Values{}
	q.Set("app_key", appKey)
	q.Set("challenge", challengeID)
	q.Set("return_to", r.returnTo)
	return r.publicURL + "/auth/channel/" + url.PathEscape(platform) + "/start?" + q.Encode(), nil
}

func (r *GatewayIdentityResolver) audit(ctx context.Context, tenantID, userID, action, detail string) {
	if r.auditLogs == nil {
		return
	}
	_ = r.auditLogs.Append(ctx, &store.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		Action:   action,
		Detail:   detail,
	})
}

func isTrustedChannelPlatform(platform Platform) bool {
	return platform == PlatformFeishu || platform == PlatformWeixin || platform == PlatformWeCom
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
