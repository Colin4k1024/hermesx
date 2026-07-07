package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/skills"
)

// UserSkillHandler manages per-user skills stored in object storage.
type UserSkillHandler struct {
	minio objstore.ObjectStore
}

// NewUserSkillHandler creates a handler for user-scoped skill CRUD.
func NewUserSkillHandler(mc objstore.ObjectStore) *UserSkillHandler {
	return &UserSkillHandler{minio: mc}
}

// ServeHTTP handles /v1/user-skills and /v1/user-skills/{name} routes.
func (h *UserSkillHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.minio == nil {
		http.Error(w, "skills storage not configured", http.StatusServiceUnavailable)
		return
	}

	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/user-skills")
	path = strings.TrimPrefix(path, "/")
	skillName := strings.TrimSuffix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if skillName == "" {
			h.list(w, r, ac)
		} else {
			h.get(w, r, ac, skillName)
		}
	case http.MethodPut:
		if skillName != "" {
			h.put(w, r, ac, skillName)
		} else {
			http.Error(w, "skill name required", http.StatusBadRequest)
		}
	case http.MethodDelete:
		if skillName != "" {
			h.delete(w, r, ac, skillName)
		} else {
			http.Error(w, "skill name required", http.StatusBadRequest)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *UserSkillHandler) list(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext) {
	loader := skills.NewMinIOUserSkillLoader(h.minio, ac.TenantID, ac.Identity)
	entries, err := loader.LoadAll(r.Context())
	if err != nil {
		slog.Error("list user skills failed", "user", ac.Identity, "error", err)
		http.Error(w, "failed to list skills", http.StatusInternalServerError)
		return
	}

	items := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, map[string]string{
			"name":        e.Meta.Name,
			"description": e.Meta.Description,
			"version":     e.Meta.Version,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"skills": items,
		"total":  len(items),
	})
}

func (h *UserSkillHandler) get(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, name string) {
	key := userSkillKey(ac.TenantID, ac.Identity, name)
	data, err := h.minio.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (h *UserSkillHandler) put(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, name string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "body is required (SKILL.md content)", http.StatusBadRequest)
		return
	}

	key := userSkillKey(ac.TenantID, ac.Identity, name)
	if err := h.minio.PutObject(r.Context(), key, body); err != nil {
		slog.Error("put user skill failed", "user", ac.Identity, "skill", name, "error", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	slog.Info("user skill uploaded", "user", ac.Identity, "skill", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":   name,
		"status": "ok",
	})
}

func (h *UserSkillHandler) delete(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, name string) {
	key := userSkillKey(ac.TenantID, ac.Identity, name)
	if err := h.minio.DeleteObject(r.Context(), key); err != nil {
		slog.Error("delete user skill failed", "user", ac.Identity, "skill", name, "error", err)
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// userSkillKey returns the MinIO path for a user's skill.
func userSkillKey(tenantID, userID, skillName string) string {
	return fmt.Sprintf("%s/users/%s/%s/SKILL.md", tenantID, userID, skillName)
}
