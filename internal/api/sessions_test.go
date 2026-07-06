package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// --- Active Sessions endpoint tests ---

func TestHandleListActiveSessions_Unauthorized(t *testing.T) {
	h := &chatHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/active", nil)
	w := httptest.NewRecorder()

	h.handleListActiveSessions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleListActiveSessions_NilStore_ReturnsEmpty(t *testing.T) {
	h := &chatHandler{} // nil store
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/active", ac)
	w := httptest.NewRecorder()

	h.handleListActiveSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty body")
	}
}

// --- Session Artifacts endpoint tests ---

func TestHandleListSessionArtifacts_Unauthorized(t *testing.T) {
	h := &chatHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/sess-1/artifacts", nil)
	w := httptest.NewRecorder()

	h.handleListSessionArtifacts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleListSessionArtifacts_MissingSessionID(t *testing.T) {
	h := &chatHandler{}
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/", ac)
	w := httptest.NewRecorder()

	h.handleListSessionArtifacts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleListSessionArtifacts_NilStore_ReturnsEmpty(t *testing.T) {
	h := &chatHandler{} // nil store
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/sess-1/artifacts", ac)
	w := httptest.NewRecorder()

	h.handleListSessionArtifacts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- Dispatcher tests (handleGetSessionMessages routing) ---

func TestHandleGetSessionMessages_DispatchActive(t *testing.T) {
	h := &chatHandler{} // nil store returns empty list for active sessions
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/active", ac)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	// Should dispatch to handleListActiveSessions which returns 200 (nil store).
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (dispatched to active sessions), got %d", w.Code)
	}
}

func TestHandleGetSessionMessages_DispatchArtifacts(t *testing.T) {
	h := &chatHandler{} // nil store returns empty artifacts
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/sess-1/artifacts", ac)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	// Should dispatch to handleListSessionArtifacts which returns 200 (nil store).
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (dispatched to artifacts), got %d", w.Code)
	}
}

func TestHandleGetSessionMessages_DispatchMessages(t *testing.T) {
	h := &chatHandler{} // nil store returns empty messages
	ac := &auth.AuthContext{
		TenantID: "tenant-1",
		Identity: "user-1",
		Roles:    []string{"user"},
	}
	req := memoryReq(http.MethodGet, "/v1/sessions/sess-1/messages", ac)
	w := httptest.NewRecorder()

	h.handleGetSessionMessages(w, req)

	// Should dispatch to normal message listing path, which returns 200 (nil store).
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (dispatched to messages), got %d", w.Code)
	}
}

func TestHandleGetSessionMessages_EmptyPath(t *testing.T) {
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

// --- sessionStatus tests ---

func TestSessionStatus(t *testing.T) {
	tests := []struct {
		name string
		sess *store.Session
		want string
	}{
		{
			name: "no ended_at is running",
			sess: &store.Session{},
			want: "running",
		},
		{
			name: "ended without error reason is completed",
			sess: &store.Session{
				EndedAt:   timePtr(time.Now()),
				EndReason: "stop",
			},
			want: "completed",
		},
		{
			name: "ended with error reason is failed",
			sess: &store.Session{
				EndedAt:   timePtr(time.Now()),
				EndReason: "error",
			},
			want: "failed",
		},
		{
			name: "ended with zero time is running",
			sess: &store.Session{
				EndedAt: timePtr(time.Time{}),
			},
			want: "running",
		},
		{
			name: "ended with empty reason is completed",
			sess: &store.Session{
				EndedAt:   timePtr(time.Now()),
				EndReason: "",
			},
			want: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionStatus(tt.sess); got != tt.want {
				t.Errorf("sessionStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
