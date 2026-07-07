package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// --- Mock FileEntryStore ---

type mockFileEntryStore struct {
	entries  map[string]*store.FileEntry // id -> entry
	usage    int64
	createFn func(entry *store.FileEntry) error
}

func newMockFileEntryStore() *mockFileEntryStore {
	return &mockFileEntryStore{entries: make(map[string]*store.FileEntry)}
}

func (m *mockFileEntryStore) List(_ context.Context, tenantID, userID string) ([]*store.FileEntry, error) {
	var result []*store.FileEntry
	for _, e := range m.entries {
		if e.TenantID == tenantID && e.UserID == userID && e.DeletedAt == nil {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockFileEntryStore) Get(_ context.Context, tenantID, userID, id string) (*store.FileEntry, error) {
	e, ok := m.entries[id]
	if !ok || e.TenantID != tenantID || e.UserID != userID || e.DeletedAt != nil {
		return nil, store.ErrNotFound
	}
	return e, nil
}

func (m *mockFileEntryStore) Create(_ context.Context, entry *store.FileEntry) error {
	if m.createFn != nil {
		return m.createFn(entry)
	}
	m.entries[entry.ID] = entry
	m.usage += entry.SizeBytes
	return nil
}

func (m *mockFileEntryStore) Delete(_ context.Context, tenantID, userID, id string) error {
	e, ok := m.entries[id]
	if !ok || e.TenantID != tenantID || e.UserID != userID {
		return store.ErrNotFound
	}
	m.usage -= e.SizeBytes
	delete(m.entries, id)
	return nil
}

func (m *mockFileEntryStore) GetUserStorageUsage(_ context.Context, _, _ string) (int64, error) {
	return m.usage, nil
}

// --- Mock ObjectStore ---

type mockFileObjectStore struct {
	objects map[string][]byte
}

func newMockFileObjectStore() *mockFileObjectStore {
	return &mockFileObjectStore{objects: make(map[string][]byte)}
}

func (m *mockFileObjectStore) EnsureBucket(_ context.Context) error { return nil }
func (m *mockFileObjectStore) Bucket() string                       { return "test-bucket" }
func (m *mockFileObjectStore) Ping(_ context.Context) error         { return nil }
func (m *mockFileObjectStore) GetObject(_ context.Context, key string) ([]byte, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return data, nil
}
func (m *mockFileObjectStore) PutObject(_ context.Context, key string, data []byte) error {
	m.objects[key] = data
	return nil
}
func (m *mockFileObjectStore) PutObjectWithContentType(_ context.Context, key string, data []byte, _ string) error {
	m.objects[key] = data
	return nil
}
func (m *mockFileObjectStore) DeleteObject(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}
func (m *mockFileObjectStore) ObjectExists(_ context.Context, key string) (bool, error) {
	_, ok := m.objects[key]
	return ok, nil
}
func (m *mockFileObjectStore) ListObjects(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// --- Test helpers ---

type mockFileStore struct {
	fileEntries store.FileEntryStore
}

func (m *mockFileStore) Sessions() store.SessionStore                   { return nil }
func (m *mockFileStore) Messages() store.MessageStore                   { return nil }
func (m *mockFileStore) Users() store.UserStore                         { return nil }
func (m *mockFileStore) Tenants() store.TenantStore                     { return nil }
func (m *mockFileStore) AuditLogs() store.AuditLogStore                 { return nil }
func (m *mockFileStore) APIKeys() store.APIKeyStore                     { return nil }
func (m *mockFileStore) Memories() store.MemoryStore                    { return nil }
func (m *mockFileStore) UserProfiles() store.UserProfileStore           { return nil }
func (m *mockFileStore) CronJobs() store.CronJobStore                   { return nil }
func (m *mockFileStore) Roles() store.RoleStore                         { return nil }
func (m *mockFileStore) PricingRules() store.PricingRuleStore           { return nil }
func (m *mockFileStore) ExecutionReceipts() store.ExecutionReceiptStore { return nil }
func (m *mockFileStore) FileEntries() store.FileEntryStore              { return m.fileEntries }
func (m *mockFileStore) Workflows() store.WorkflowStore                 { return nil }
func (m *mockFileStore) AgentProfiles() store.AgentProfileStore         { return nil }
func (m *mockFileStore) Close() error                                   { return nil }
func (m *mockFileStore) Migrate(_ context.Context) error                { return nil }

func withAuthContext(next http.Handler, tenantID, userID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := &auth.AuthContext{
			Identity: userID,
			UserID:   userID,
			TenantID: tenantID,
			Roles:    []string{"user"},
		}
		ctx := auth.WithContext(r.Context(), ac)
		ctx = middleware.WithTenant(ctx, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- Tests ---

func TestFileHandler_Upload(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	// Build multipart form
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("hello world"))
	_ = mw.WriteField("path", "docs/test.txt")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var entry store.FileEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if entry.Path != "docs/test.txt" {
		t.Errorf("expected path docs/test.txt, got %s", entry.Path)
	}
	if entry.SizeBytes != 11 {
		t.Errorf("expected size 11, got %d", entry.SizeBytes)
	}
	if entry.SHA256 == "" {
		t.Error("expected non-empty SHA256")
	}

	// Verify object in MinIO
	key := "tenant-1/user-1/workspace/docs/test.txt"
	data, ok := objStore.objects[key]
	if !ok {
		t.Fatalf("expected object at %s", key)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(data))
	}
}

func TestFileHandler_Upload_PathTraversal(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "evil.txt")
	_, _ = part.Write([]byte("bad"))
	_ = mw.WriteField("path", "../../../etc/passwd")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFileHandler_Upload_AbsolutePath(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "evil.txt")
	_, _ = part.Write([]byte("bad"))
	_ = mw.WriteField("path", "/etc/passwd")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for absolute path, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFileHandler_List(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	// Pre-populate some entries
	fileStore.entries["id-1"] = &store.FileEntry{
		ID: "id-1", TenantID: "tenant-1", UserID: "user-1",
		Path: "doc1.txt", SizeBytes: 100,
	}
	fileStore.entries["id-2"] = &store.FileEntry{
		ID: "id-2", TenantID: "tenant-1", UserID: "user-1",
		Path: "doc2.txt", SizeBytes: 200,
	}
	// Different user — should not appear
	fileStore.entries["id-3"] = &store.FileEntry{
		ID: "id-3", TenantID: "tenant-1", UserID: "user-2",
		Path: "other.txt", SizeBytes: 300,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Files []*store.FileEntry `json:"files"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 files, got %d", resp.Total)
	}
}

func TestFileHandler_List_Empty(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Files []*store.FileEntry `json:"files"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected 0 files, got %d", resp.Total)
	}
	if resp.Files == nil {
		t.Error("expected non-nil empty array")
	}
}

func TestFileHandler_Download(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	// Pre-populate
	fileStore.entries["id-1"] = &store.FileEntry{
		ID: "id-1", TenantID: "tenant-1", UserID: "user-1",
		Path: "report.md", MinIOKey: "tenant-1/user-1/workspace/report.md",
		MIMEType: "text/markdown", SizeBytes: 10,
	}
	objStore.objects["tenant-1/user-1/workspace/report.md"] = []byte("# Report")

	req := httptest.NewRequest(http.MethodGet, "/v1/files/id-1/download", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "# Report" {
		t.Errorf("expected '# Report', got '%s'", rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/markdown" {
		t.Errorf("expected text/markdown, got %s", ct)
	}
}

func TestFileHandler_Download_NotFound(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/files/nonexistent/download", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFileHandler_Delete(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	fileStore.entries["id-1"] = &store.FileEntry{
		ID: "id-1", TenantID: "tenant-1", UserID: "user-1",
		Path: "old.txt", MinIOKey: "tenant-1/user-1/workspace/old.txt",
		SizeBytes: 50,
	}
	fileStore.usage = 50
	objStore.objects["tenant-1/user-1/workspace/old.txt"] = []byte("old data")

	req := httptest.NewRequest(http.MethodDelete, "/v1/files/id-1", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify entry removed
	if _, ok := fileStore.entries["id-1"]; ok {
		t.Error("expected entry to be deleted")
	}
	// Verify MinIO object removed
	if _, ok := objStore.objects["tenant-1/user-1/workspace/old.txt"]; ok {
		t.Error("expected MinIO object to be deleted")
	}
}

func TestFileHandler_Delete_NotFound(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	req := httptest.NewRequest(http.MethodDelete, "/v1/files/nonexistent", nil)
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFileHandler_Promote(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	// Simulate a session file in MinIO
	objStore.objects["tenant-1/user-1/sessions/sess-abc/output.json"] = []byte(`{"result":"ok"}`)

	body := `{"session_id":"sess-abc","source_path":"output.json"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/files/promote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var entry store.FileEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry.Path != "output.json" {
		t.Errorf("expected path output.json, got %s", entry.Path)
	}
	if entry.SourceSession != "sess-abc" {
		t.Errorf("expected source_session sess-abc, got %s", entry.SourceSession)
	}

	// Verify workspace copy exists
	wsKey := "tenant-1/user-1/workspace/output.json"
	if _, ok := objStore.objects[wsKey]; !ok {
		t.Error("expected workspace copy to exist")
	}
}

func TestFileHandler_Promote_SourceNotFound(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	body := `{"session_id":"sess-abc","source_path":"nonexistent.json"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/files/promote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFileHandler_Promote_WithDestPath(t *testing.T) {
	fileStore := newMockFileEntryStore()
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	objStore.objects["tenant-1/user-1/sessions/sess-abc/data.csv"] = []byte("a,b,c")

	body := `{"session_id":"sess-abc","source_path":"data.csv","dest_path":"imports/data.csv"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/files/promote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var entry store.FileEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry.Path != "imports/data.csv" {
		t.Errorf("expected path imports/data.csv, got %s", entry.Path)
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid simple", "file.txt", false},
		{"valid nested", "docs/reports/file.md", false},
		{"empty", "", true},
		{"absolute", "/etc/passwd", true},
		{"traversal", "../secrets.txt", true},
		{"nested traversal", "docs/../../etc/passwd", true},
		{"reserved workspace", "workspace/file.txt", true},
		{"reserved sessions", "sessions/file.txt", true},
		{"null byte", "file\x00.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestFileHandler_QuotaExceeded(t *testing.T) {
	fileStore := newMockFileEntryStore()
	fileStore.usage = 512*1024*1024 - 10 // almost at 512MB limit
	objStore := newMockFileObjectStore()
	s := &mockFileStore{fileEntries: fileStore}
	handler := NewFileHandler(s, objStore)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "big.bin")
	// Write 20 bytes — exceeds remaining 10 bytes
	_, _ = part.Write([]byte("01234567890123456789"))
	_ = mw.WriteField("path", "big.bin")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	wrapped := withAuthContext(handler, "tenant-1", "user-1")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rr.Code, rr.Body.String())
	}
}
