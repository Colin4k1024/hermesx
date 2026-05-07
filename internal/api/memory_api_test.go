package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

func memoryReq(method, path string, ac *auth.AuthContext) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(auth.WithContext(req.Context(), ac))
}

func TestListMemories_NonAdmin_CannotOverrideUserID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/memories", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleListMemories(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestListMemories_Admin_CanOverrideUserID(t *testing.T) {
	h := &chatHandler{} // pool is nil → returns empty list after auth check
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "admin-user",
		Roles:    []string{"admin"},
	}
	req := memoryReq(http.MethodGet, "/v1/memories", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleListMemories(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListMemories_NoOverride_UsesIdentity(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/memories", ac)
	w := httptest.NewRecorder()

	h.handleListMemories(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDeleteMemory_NonAdmin_CannotOverrideUserID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodDelete, "/v1/memories/some-key", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleDeleteMemory(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDeleteMemory_Admin_CanOverrideUserID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "admin-user",
		Roles:    []string{"admin"},
	}
	req := memoryReq(http.MethodDelete, "/v1/memories/some-key", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleDeleteMemory(w, req)

	// Should get 503 (pool is nil) rather than 403
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (pool nil), got %d", w.Code)
	}
}

func TestListUserSessions_NonAdmin_CannotOverrideUserID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleListUserSessions(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestListUserSessions_Admin_CanOverrideUserID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "admin-user",
		Roles:    []string{"admin"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions", ac)
	req.Header.Set("X-Hermes-User-Id", "other-user")
	w := httptest.NewRecorder()

	h.handleListUserSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetSessionMessages_AdminBypassesOwnership(t *testing.T) {
	h := &chatHandler{} // pool nil → returns empty after ownership check skip
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "admin-user",
		Roles:    []string{"admin"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/any-session-id/messages", ac)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (admin bypasses ownership), got %d", w.Code)
	}
}

func TestGetSessionMessages_Unauthorized(t *testing.T) {
	h := &chatHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/sess-1/messages", nil)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetSessionMessages_MissingSessionID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/", ac)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
