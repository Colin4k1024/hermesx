package auth

import (
	"fmt"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/store"
)

const (
	ChannelSessionCookie = "hx_session"
	ChannelCSRFCookie    = "hx_csrf"
	ChannelAuthMethod    = "channel_session"
)

// ChannelSessionExtractor authenticates opaque browser sessions created by
// trusted channel login callbacks.
type ChannelSessionExtractor struct {
	sessions store.BrowserSessionStore
}

func NewChannelSessionExtractor(sessions store.BrowserSessionStore) *ChannelSessionExtractor {
	return &ChannelSessionExtractor{sessions: sessions}
}

func (e *ChannelSessionExtractor) Extract(r *http.Request) (*AuthContext, error) {
	if e == nil || e.sessions == nil {
		return nil, nil
	}
	cookie, err := r.Cookie(ChannelSessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, nil
	}

	session, err := e.sessions.GetByHash(r.Context(), HashKey(cookie.Value))
	if err != nil {
		return nil, fmt.Errorf("channel session invalid: %w", err)
	}
	_ = e.sessions.Touch(r.Context(), session.ID)

	return &AuthContext{
		Identity:   session.UserID,
		UserID:     session.UserID,
		TenantID:   session.TenantID,
		Roles:      []string{"user"},
		Scopes:     []string{"read", "write", "execute"},
		AuthMethod: ChannelAuthMethod,
		SessionID:  session.ID,
		CSRFHash:   session.CSRFTokenHash,
	}, nil
}
