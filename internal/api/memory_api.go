package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type memoryEntry struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

func (h *chatHandler) handleListMemories(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Use ac.Identity by default. Admin users can override via X-Hermes-User-Id header
	// to query other users' memories (used by integration tests and admin tools).
	userID := ac.Identity
	if override := r.Header.Get("X-Hermes-User-Id"); override != "" && ac.HasRole("admin") {
		userID = override
	}

	if h.store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"memories": []memoryEntry{}, "count": 0})
		return
	}

	ctx := r.Context()
	raw, err := h.store.Memories().List(ctx, ac.TenantID, userID)
	if err != nil {
		http.Error(w, "failed to query memories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	entries := make([]memoryEntry, 0, len(raw))
	for _, e := range raw {
		entries = append(entries, memoryEntry{Key: e.Key, Content: e.Content})
	}

	// Also include user profile if available.
	var profile string
	if h.store.UserProfiles() != nil {
		profile, _ = h.store.UserProfiles().Get(ctx, ac.TenantID, userID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id": ac.TenantID,
		"user_id":   userID,
		"memories":  entries,
		"count":     len(entries),
		"profile":   profile,
	})
}

func (h *chatHandler) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := ac.Identity
	if override := r.Header.Get("X-Hermes-User-Id"); override != "" && ac.HasRole("admin") {
		userID = override
	}

	key := strings.TrimPrefix(r.URL.Path, "/v1/memories/")
	if key == "" {
		http.Error(w, "memory key required", http.StatusBadRequest)
		return
	}

	if h.store == nil {
		http.Error(w, "memory store not available", http.StatusServiceUnavailable)
		return
	}

	if err := h.store.Memories().Delete(r.Context(), ac.TenantID, userID, key); err != nil {
		http.Error(w, "failed to delete memory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *chatHandler) handleListUserSessions(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := ac.Identity
	if override := r.Header.Get("X-Hermes-User-Id"); override != "" && ac.HasRole("admin") {
		userID = override
	}

	if h.store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sessions": []any{}})
		return
	}

	sessions, _, err := h.store.Sessions().List(r.Context(), ac.TenantID, store.ListOptions{
		UserID: userID,
		Limit:  50,
	})
	if err != nil {
		http.Error(w, "failed to query sessions", http.StatusInternalServerError)
		return
	}

	type sessionEntry struct {
		ID           string  `json:"id"`
		StartedAt    string  `json:"started_at"`
		EndedAt      *string `json:"ended_at,omitempty"`
		MessageCount int     `json:"message_count"`
	}

	result := make([]sessionEntry, 0, len(sessions))
	for _, s := range sessions {
		msgCount, _ := h.store.Messages().CountBySession(r.Context(), ac.TenantID, s.ID)
		se := sessionEntry{
			ID:           s.ID,
			StartedAt:    s.StartedAt.Format(time.RFC3339),
			MessageCount: msgCount,
		}
		if s.EndedAt != nil && !s.EndedAt.IsZero() {
			ea := s.EndedAt.Format(time.RFC3339)
			se.EndedAt = &ea
		}
		result = append(result, se)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id": ac.TenantID,
		"user_id":   userID,
		"sessions":  result,
		"count":     len(result),
	})
}

func (h *chatHandler) handleGetSessionMessages(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	if h.store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"messages": []any{}})
		return
	}

	ctx := r.Context()

	// Session ownership check: verify session belongs to this user (admin bypasses).
	if !ac.HasRole("admin") {
		sess, err := h.store.Sessions().Get(ctx, ac.TenantID, sessionID)
		if err != nil || sess == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		if sess.UserID != ac.Identity {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	rawMsgs, err := h.store.Messages().List(ctx, ac.TenantID, sessionID, 200, 0)
	if err != nil {
		http.Error(w, "failed to query messages", http.StatusInternalServerError)
		return
	}

	type msgEntry struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}

	messages := make([]msgEntry, 0, len(rawMsgs))
	for _, m := range rawMsgs {
		messages = append(messages, msgEntry{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id":  ac.TenantID,
		"session_id": sessionID,
		"messages":   messages,
		"count":      len(messages),
	})
}
