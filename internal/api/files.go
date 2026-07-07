package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

// FileHandler serves workspace file management endpoints.
type FileHandler struct {
	store     store.Store
	objStore  objstore.ObjectStore
	maxUpload int64 // max upload size in bytes (default 100MB)
}

// NewFileHandler creates a FileHandler for workspace file management.
func NewFileHandler(s store.Store, os objstore.ObjectStore) *FileHandler {
	return &FileHandler{
		store:     s,
		objStore:  os,
		maxUpload: 100 * 1024 * 1024, // 100MB default
	}
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route by method and path suffix
	path := strings.TrimPrefix(r.URL.Path, "/v1/files")

	switch {
	case r.Method == http.MethodPost && path == "/upload":
		h.handleUpload(w, r)
	case r.Method == http.MethodGet && (path == "" || path == "/"):
		h.handleList(w, r)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/download"):
		h.handleDownload(w, r)
	case r.Method == http.MethodDelete && path != "" && path != "/":
		h.handleDelete(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/promote"):
		h.handlePromote(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleUpload processes multipart file upload to the user's workspace.
// POST /v1/files/upload
func (h *FileHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.requireAuth(w, r)
	if tenantID == "" {
		return
	}

	if err := r.ParseMultipartForm(h.maxUpload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "missing 'file' field in form")
		return
	}
	defer file.Close()

	destPath := r.FormValue("path")
	if destPath == "" {
		destPath = header.Filename
	}

	// Validate path
	if err := validateFilePath(destPath); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Read file content
	data, err := io.ReadAll(io.LimitReader(file, h.maxUpload+1))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to read upload")
		return
	}
	if int64(len(data)) > h.maxUpload {
		writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds %d byte limit", h.maxUpload))
		return
	}

	// Check quota
	if err := h.checkQuota(r.Context(), tenantID, userID, int64(len(data))); err != nil {
		if errors.Is(err, errStorageQuotaExceeded) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "quota check failed")
		return
	}

	// Compute SHA256
	hash := sha256.Sum256(data)
	sha256hex := hex.EncodeToString(hash[:])

	// Detect MIME type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(filepath.Ext(destPath))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	// Construct MinIO key
	minioKey := fmt.Sprintf("%s/%s/workspace/%s", tenantID, userID, destPath)

	// Upload to MinIO
	if err := h.objStore.PutObjectWithContentType(r.Context(), minioKey, data, mimeType); err != nil {
		slog.Error("file upload: minio put failed", "error", err, "key", minioKey)
		writeJSONError(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	// Create FileEntry
	entry := &store.FileEntry{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		Path:      destPath,
		MinIOKey:  minioKey,
		SizeBytes: int64(len(data)),
		MIMEType:  mimeType,
		SHA256:    sha256hex,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := h.store.FileEntries().Create(r.Context(), entry); err != nil {
		// Rollback MinIO object on PG failure
		_ = h.objStore.DeleteObject(r.Context(), minioKey)
		slog.Error("file upload: pg create failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to record file metadata")
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

// handleList returns the user's workspace files.
// GET /v1/files
func (h *FileHandler) handleList(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.requireAuth(w, r)
	if tenantID == "" {
		return
	}

	entries, err := h.store.FileEntries().List(r.Context(), tenantID, userID)
	if err != nil {
		slog.Error("file list failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to list files")
		return
	}

	if entries == nil {
		entries = []*store.FileEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files": entries,
		"total": len(entries),
	})
}

// handleDownload streams a workspace file from MinIO.
// GET /v1/files/{id}/download
func (h *FileHandler) handleDownload(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.requireAuth(w, r)
	if tenantID == "" {
		return
	}

	id := extractIDFromPath(r.URL.Path, "/v1/files/", "/download")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing file ID")
		return
	}

	entry, err := h.store.FileEntries().Get(r.Context(), tenantID, userID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get file")
		return
	}

	data, err := h.objStore.GetObject(r.Context(), entry.MinIOKey)
	if err != nil {
		slog.Error("file download: minio get failed", "error", err, "key", entry.MinIOKey)
		writeJSONError(w, http.StatusInternalServerError, "failed to retrieve file")
		return
	}

	w.Header().Set("Content-Type", entry.MIMEType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(entry.Path)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleDelete soft-deletes a workspace file and removes the MinIO object.
// DELETE /v1/files/{id}
func (h *FileHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.requireAuth(w, r)
	if tenantID == "" {
		return
	}

	// Extract ID from path like /v1/files/{id}
	path := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "missing file ID")
		return
	}

	entry, err := h.store.FileEntries().Get(r.Context(), tenantID, userID, path)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get file")
		return
	}

	// Soft-delete in PG
	if err := h.store.FileEntries().Delete(r.Context(), tenantID, userID, path); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	// Delete from MinIO (best effort — orphan cleanup handles leftovers)
	if err := h.objStore.DeleteObject(r.Context(), entry.MinIOKey); err != nil {
		slog.Warn("file delete: minio delete failed (will be cleaned by orphan job)",
			"error", err, "key", entry.MinIOKey)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      path,
	})
}

// handlePromote copies a session file to the user's workspace.
// POST /v1/files/{id}/promote
func (h *FileHandler) handlePromote(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.requireAuth(w, r)
	if tenantID == "" {
		return
	}

	// Extract session file ID from path — for promote, the "id" is actually the session file path
	// Expected body: { "session_id": "...", "source_path": "...", "dest_path": "..." }
	var req struct {
		SessionID  string `json:"session_id"`
		SourcePath string `json:"source_path"`
		DestPath   string `json:"dest_path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.SessionID == "" || req.SourcePath == "" {
		writeJSONError(w, http.StatusBadRequest, "session_id and source_path are required")
		return
	}

	if err := validateFilePath(req.SourcePath); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid source_path: "+err.Error())
		return
	}

	destPath := req.DestPath
	if destPath == "" {
		destPath = req.SourcePath
	}
	if err := validateFilePath(destPath); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid dest_path: "+err.Error())
		return
	}

	// Source MinIO key: session sandbox
	srcKey := fmt.Sprintf("%s/%s/sessions/%s/%s", tenantID, userID, req.SessionID, req.SourcePath)

	// Check if source exists
	exists, err := h.objStore.ObjectExists(r.Context(), srcKey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to check source file")
		return
	}
	if !exists {
		writeJSONError(w, http.StatusNotFound, "session file not found")
		return
	}

	// Read source object
	data, err := h.objStore.GetObject(r.Context(), srcKey)
	if err != nil {
		slog.Error("promote: minio get failed", "error", err, "key", srcKey)
		writeJSONError(w, http.StatusInternalServerError, "failed to read source file")
		return
	}

	// Check quota
	if err := h.checkQuota(r.Context(), tenantID, userID, int64(len(data))); err != nil {
		if errors.Is(err, errStorageQuotaExceeded) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "quota check failed")
		return
	}

	// Compute SHA256
	hash := sha256.Sum256(data)
	sha256hex := hex.EncodeToString(hash[:])

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(destPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Destination MinIO key: workspace
	dstKey := fmt.Sprintf("%s/%s/workspace/%s", tenantID, userID, destPath)

	// Copy to workspace
	if err := h.objStore.PutObjectWithContentType(r.Context(), dstKey, data, mimeType); err != nil {
		slog.Error("promote: minio put failed", "error", err, "key", dstKey)
		writeJSONError(w, http.StatusInternalServerError, "failed to copy file to workspace")
		return
	}

	// Create FileEntry
	entry := &store.FileEntry{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		UserID:        userID,
		Path:          destPath,
		MinIOKey:      dstKey,
		SizeBytes:     int64(len(data)),
		MIMEType:      mimeType,
		SHA256:        sha256hex,
		SourceSession: req.SessionID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := h.store.FileEntries().Create(r.Context(), entry); err != nil {
		// Rollback MinIO object on PG failure
		_ = h.objStore.DeleteObject(r.Context(), dstKey)
		slog.Error("promote: pg create failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to record promoted file")
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

// --- Helpers ---

// requireAuth extracts tenant and user IDs from the request context.
func (h *FileHandler) requireAuth(w http.ResponseWriter, r *http.Request) (string, string) {
	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		writeJSONError(w, http.StatusBadRequest, "tenant context required")
		return "", ""
	}

	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return "", ""
	}

	userID := ac.UserID
	if userID == "" {
		userID = ac.Identity // fallback for API key auth
	}
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "user identity required")
		return "", ""
	}

	return tenantID, userID
}

// checkQuota verifies that the user has not exceeded their storage quota.
func (h *FileHandler) checkQuota(ctx context.Context, tenantID, userID string, additionalBytes int64) error {
	currentBytes, err := h.store.FileEntries().GetUserStorageUsage(ctx, tenantID, userID)
	if err != nil {
		return fmt.Errorf("get storage usage: %w", err)
	}

	// Default quota: 512MB if tenant quota not configured.
	// TODO: integrate with governance.Quota.MaxStorageMB when governance client is wired.
	maxStorageMB := int64(512)
	maxStorageBytes := maxStorageMB * 1024 * 1024

	if currentBytes+additionalBytes > maxStorageBytes {
		return errStorageQuotaExceeded
	}
	return nil
}

var errStorageQuotaExceeded = fmt.Errorf("storage quota exceeded")

// validateFilePath checks that a user-supplied path is safe.
func validateFilePath(p string) error {
	if p == "" {
		return fmt.Errorf("path is required")
	}

	// Reject absolute paths
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	// Clean and check for traversal
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal is not allowed")
	}

	// Reject reserved prefixes
	if strings.HasPrefix(cleaned, "workspace/") || strings.HasPrefix(cleaned, "sessions/") {
		return fmt.Errorf("reserved path prefix")
	}

	// Reject null bytes
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("invalid path character")
	}

	return nil
}

// extractIDFromPath extracts a path segment between prefix and suffix.
// e.g., extractIDFromPath("/v1/files/abc123/download", "/v1/files/", "/download") => "abc123"
func extractIDFromPath(path, prefix, suffix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, suffix)
	rest = strings.TrimSuffix(rest, "/")
	return rest
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
