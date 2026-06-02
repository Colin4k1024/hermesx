package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type fakeBrowserSessionStore struct {
	session *store.BrowserSession
	touched string
	err     error
}

func (s *fakeBrowserSessionStore) Create(_ context.Context, _ *store.BrowserSession) error {
	return nil
}

func (s *fakeBrowserSessionStore) GetByHash(_ context.Context, tokenHash string) (*store.BrowserSession, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.session == nil || s.session.TokenHash != tokenHash {
		return nil, store.ErrNotFound
	}
	return s.session, nil
}

func (s *fakeBrowserSessionStore) Touch(_ context.Context, id string) error {
	s.touched = id
	return nil
}

func (s *fakeBrowserSessionStore) Revoke(_ context.Context, _ string) error { return nil }

func (s *fakeBrowserSessionStore) RevokeByUser(_ context.Context, _, _ string) error {
	return nil
}

func TestChannelSessionExtractor(t *testing.T) {
	raw := "session-token"
	sessions := &fakeBrowserSessionStore{session: &store.BrowserSession{
		ID:            "sess-1",
		TenantID:      "tenant-1",
		UserID:        "user-1",
		TokenHash:     HashKey(raw),
		CSRFTokenHash: HashKey("csrf-token"),
		ExpiresAt:     time.Now().Add(time.Hour),
	}}
	extractor := NewChannelSessionExtractor(sessions)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: ChannelSessionCookie, Value: raw})
	ac, err := extractor.Extract(req)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if ac == nil {
		t.Fatal("expected auth context")
	}
	if ac.AuthMethod != ChannelAuthMethod || ac.TenantID != "tenant-1" || ac.UserID != "user-1" {
		t.Fatalf("unexpected auth context: %+v", ac)
	}
	if ac.SessionID != "sess-1" || ac.CSRFHash != HashKey("csrf-token") {
		t.Fatalf("missing session csrf fields: %+v", ac)
	}
	if sessions.touched != "sess-1" {
		t.Fatalf("session touch id = %q, want sess-1", sessions.touched)
	}
}

func TestChannelSessionExtractor_NoCookieAndInvalidCookie(t *testing.T) {
	extractor := NewChannelSessionExtractor(&fakeBrowserSessionStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	ac, err := extractor.Extract(req)
	if err != nil || ac != nil {
		t.Fatalf("no cookie ac=%v err=%v, want nil nil", ac, err)
	}

	req.AddCookie(&http.Cookie{Name: ChannelSessionCookie, Value: "missing"})
	ac, err = extractor.Extract(req)
	if err != nil || ac != nil {
		t.Fatalf("invalid cookie ac=%v err=%v, want nil nil (unknown session treated as unauthenticated)", ac, err)
	}
}
