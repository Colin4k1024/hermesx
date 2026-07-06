package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// activeSessionEntry is the response type for GET /v1/sessions/active.
type activeSessionEntry struct {
	ID           string  `json:"id"`
	Title        string  `json:"title,omitempty"`
	Status       string  `json:"status"`
	StartedAt    string  `json:"started_at"`
	UpdatedAt    string  `json:"updated_at"`
	ArtifactCount int    `json:"artifact_count"`
}

// sessionArtifact is the response type for GET /v1/sessions/{id}/artifacts.
type sessionArtifact struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

// sessionStatus determines the session status from its fields.
func sessionStatus(s *store.Session) string {
	if s.EndedAt != nil && !s.EndedAt.IsZero() {
		if s.EndReason == "error" {
			return "failed"
		}
		return "completed"
	}
	// Sessions that have not ended are considered "running".
	return "running"
}

// handleListActiveSessions handles GET /v1/sessions/active.
// Returns sessions belonging to the current user that have no end time
// (i.e., still active/running), ordered by most recently started first.
func (h *chatHandler) handleListActiveSessions(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(map[string]any{"sessions": []activeSessionEntry{}, "count": 0})
		return
	}

	ctx := r.Context()
	sessions, _, err := h.store.Sessions().List(ctx, ac.TenantID, store.ListOptions{
		UserID: userID,
		Limit:  100,
	})
	if err != nil {
		http.Error(w, "failed to query sessions", http.StatusInternalServerError)
		return
	}

	result := make([]activeSessionEntry, 0, len(sessions))
	for _, s := range sessions {
		status := sessionStatus(s)
		if status != "running" {
			continue
		}
		updatedAt := s.StartedAt.Format(time.RFC3339)

		// Count messages as a proxy for activity; use message count from session.
		artifactCount := 0
		if h.skillsClient != nil {
			prefix := "artifacts/" + ac.TenantID + "/" + s.ID + "/"
			keys, err := h.skillsClient.ListObjects(ctx, prefix)
			if err == nil {
				artifactCount = len(keys)
			}
		}

		result = append(result, activeSessionEntry{
			ID:            s.ID,
			Title:         s.Title,
			Status:        status,
			StartedAt:     s.StartedAt.Format(time.RFC3339),
			UpdatedAt:     updatedAt,
			ArtifactCount: artifactCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"sessions": result,
		"count":    len(result),
	})
}

// handleListSessionArtifacts handles GET /v1/sessions/{id}/artifacts.
// Lists artifacts generated during a session by querying the ObjectStore
// with the session-scoped prefix.
func (h *chatHandler) handleListSessionArtifacts(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(map[string]any{"artifacts": []sessionArtifact{}, "count": 0})
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

	if h.skillsClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"artifacts": []sessionArtifact{}, "count": 0})
		return
	}

	prefix := "artifacts/" + ac.TenantID + "/" + sessionID + "/"
	keys, err := h.skillsClient.ListObjects(ctx, prefix)
	if err != nil {
		http.Error(w, "failed to list artifacts", http.StatusInternalServerError)
		return
	}

	artifacts := make([]sessionArtifact, 0, len(keys))
	for _, key := range keys {
		name := key
		if idx := strings.LastIndex(key, "/"); idx >= 0 {
			name = key[idx+1:]
		}

		artType := "file"
		if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".svg") {
			artType = "image"
		} else if strings.HasSuffix(name, ".json") {
			artType = "data"
		} else if strings.HasSuffix(name, ".csv") || strings.HasSuffix(name, ".xlsx") {
			artType = "spreadsheet"
		}

		artifacts = append(artifacts, sessionArtifact{
			Name:      name,
			Type:      artType,
			URL:       "/v1/objects/" + key,
			SizeBytes: 0,
			CreatedAt: time.Now().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id": sessionID,
		"artifacts":  artifacts,
		"count":      len(artifacts),
	})
}
