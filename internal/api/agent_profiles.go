package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

// AgentProfileHandler handles CRUD for named agent configurations.
type AgentProfileHandler struct {
	store store.Store
	minio objstore.ObjectStore
}

// NewAgentProfileHandler creates a new handler for agent profile operations.
func NewAgentProfileHandler(s store.Store, mc objstore.ObjectStore) *AgentProfileHandler {
	return &AgentProfileHandler{store: s, minio: mc}
}

type createAgentProfileReq struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Model          string   `json:"model,omitempty"`
	SelectedSkills []string `json:"selected_skills,omitempty"`
	SoulContent    string   `json:"soul_content,omitempty"`
}

type updateAgentProfileReq struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Model          string   `json:"model,omitempty"`
	SelectedSkills []string `json:"selected_skills,omitempty"`
}

// ServeHTTP handles /v1/agent-profiles and /v1/agent-profiles/{id} routes.
func (h *AgentProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract profile ID from path if present.
	path := strings.TrimPrefix(r.URL.Path, "/v1/agent-profiles")
	path = strings.TrimPrefix(path, "/")
	profileID := strings.TrimSuffix(path, "/")

	// Handle sub-paths like /{id}/default, /{id}/soul.
	var subPath string
	if idx := strings.Index(profileID, "/"); idx >= 0 {
		subPath = profileID[idx+1:]
		profileID = profileID[:idx]
	}

	switch r.Method {
	case http.MethodGet:
		if profileID == "" {
			h.list(w, r, ac)
		} else if subPath == "soul" {
			h.getSoul(w, r, ac, profileID)
		} else if subPath == "" {
			h.get(w, r, ac, profileID)
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	case http.MethodPost:
		if profileID == "" {
			h.create(w, r, ac)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case http.MethodPut:
		if subPath == "default" {
			h.setDefault(w, r, ac, profileID)
		} else if subPath == "soul" {
			h.updateSoul(w, r, ac, profileID)
		} else if profileID != "" {
			h.update(w, r, ac, profileID)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case http.MethodDelete:
		if profileID != "" && subPath == "" {
			h.delete(w, r, ac, profileID)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AgentProfileHandler) list(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext) {
	profiles, err := h.store.AgentProfiles().List(r.Context(), ac.TenantID, ac.Identity)
	if err != nil {
		slog.Error("list agent profiles failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if profiles == nil {
		profiles = []*store.AgentProfile{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"profiles": profiles, "count": len(profiles)})
}

func (h *AgentProfileHandler) get(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	profile, err := h.store.AgentProfiles().Get(r.Context(), ac.TenantID, ac.Identity, profileID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		slog.Error("get agent profile failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Load soul content from MinIO.
	soulContent := ""
	if h.minio != nil {
		soulKey := agentSoulKey(ac.TenantID, ac.Identity, profileID)
		data, err := h.minio.GetObject(r.Context(), soulKey)
		if err == nil {
			soulContent = string(data)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"profile": profile, "soul_content": soulContent})
}

func (h *AgentProfileHandler) create(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext) {
	var req createAgentProfileReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 128 {
		http.Error(w, "name must be at most 128 characters", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	profile := &store.AgentProfile{
		ID:             uuid.New().String(),
		TenantID:       ac.TenantID,
		UserID:         ac.Identity,
		Name:           req.Name,
		Description:    req.Description,
		Model:          req.Model,
		SelectedSkills: req.SelectedSkills,
	}

	if err := h.store.AgentProfiles().Create(ctx, profile); err != nil {
		slog.Error("create agent profile failed", "error", err)
		http.Error(w, "failed to create profile", http.StatusInternalServerError)
		return
	}

	// Write SOUL.md to MinIO.
	if h.minio != nil {
		soulContent := req.SoulContent
		if soulContent == "" {
			soulContent = skills.DefaultSoulContent()
		}
		soulKey := agentSoulKey(ac.TenantID, ac.Identity, profile.ID)
		if err := h.minio.PutObject(ctx, soulKey, []byte(soulContent)); err != nil {
			slog.Error("write agent soul failed", "key", soulKey, "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"profile": profile})
}

func (h *AgentProfileHandler) update(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	var req updateAgentProfileReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	existing, err := h.store.AgentProfiles().Get(ctx, ac.TenantID, ac.Identity, profileID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	existing.Model = req.Model
	if req.SelectedSkills != nil {
		existing.SelectedSkills = req.SelectedSkills
	}

	if err := h.store.AgentProfiles().Update(ctx, existing); err != nil {
		slog.Error("update agent profile failed", "error", err)
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"profile": existing})
}

func (h *AgentProfileHandler) delete(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	ctx := r.Context()
	if err := h.store.AgentProfiles().Delete(ctx, ac.TenantID, ac.Identity, profileID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		slog.Error("delete agent profile failed", "error", err)
		http.Error(w, "failed to delete profile", http.StatusInternalServerError)
		return
	}

	// Delete SOUL.md from MinIO.
	if h.minio != nil {
		soulKey := agentSoulKey(ac.TenantID, ac.Identity, profileID)
		_ = h.minio.DeleteObject(ctx, soulKey)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AgentProfileHandler) setDefault(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.AgentProfiles().SetDefault(r.Context(), ac.TenantID, ac.Identity, profileID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		slog.Error("set default agent profile failed", "error", err)
		http.Error(w, "failed to set default", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AgentProfileHandler) getSoul(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	if h.minio == nil {
		http.Error(w, "object storage not configured", http.StatusServiceUnavailable)
		return
	}
	soulKey := agentSoulKey(ac.TenantID, ac.Identity, profileID)
	data, err := h.minio.GetObject(r.Context(), soulKey)
	if err != nil {
		http.Error(w, "soul not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (h *AgentProfileHandler) updateSoul(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, profileID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.minio == nil {
		http.Error(w, "object storage not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Verify profile exists.
	if _, err := h.store.AgentProfiles().Get(r.Context(), ac.TenantID, ac.Identity, profileID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	soulKey := agentSoulKey(ac.TenantID, ac.Identity, profileID)
	if err := h.minio.PutObject(r.Context(), soulKey, []byte(req.Content)); err != nil {
		slog.Error("update agent soul failed", "key", soulKey, "error", err)
		http.Error(w, "failed to update soul", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// agentSoulKey returns the MinIO path for an agent's SOUL.md.
func agentSoulKey(tenantID, userID, agentID string) string {
	return tenantID + "/users/" + userID + "/agents/" + agentID + "/SOUL.md"
}
