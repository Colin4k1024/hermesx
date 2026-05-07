package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testManifest() *CatalogManifest {
	return &CatalogManifest{
		Version:   "2026.5.1",
		UpdatedAt: "2026-05-01T00:00:00Z",
		Models: []CatalogEntry{
			{
				Name:           "test/new-model",
				ShortName:      "newmodel",
				Provider:       "test",
				ContextLength:  256000,
				MaxOutput:      32000,
				SupportsTools:  true,
				SupportsVision: true,
				InputPrice:     1.0,
				OutputPrice:    5.0,
			},
			{
				Name:           "anthropic/claude-sonnet-4-20250514",
				Provider:       "anthropic",
				ContextLength:  200000,
				MaxOutput:      16000,
				SupportsTools:  true,
				SupportsVision: true,
			},
		},
	}
}

func TestCatalogSeedFromKnownModels(t *testing.T) {
	cfg := CatalogConfig{CacheDir: t.TempDir()}
	c := NewCatalog(cfg)

	if c.Len() == 0 {
		t.Fatal("expected catalog to be seeded with KnownModels")
	}

	entry, ok := c.Get("openai/gpt-4o")
	if !ok {
		t.Fatal("expected gpt-4o in catalog")
	}
	if entry.ContextLength != 128000 {
		t.Errorf("ContextLength=%d, want 128000", entry.ContextLength)
	}
}

func TestCatalogRefreshFromRemote(t *testing.T) {
	manifest := testManifest()
	data, _ := json.Marshal(manifest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer srv.Close()

	cfg := CatalogConfig{
		RemoteURL:    srv.URL,
		CacheDir:     t.TempDir(),
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   srv.Client(),
	}

	c := NewCatalog(cfg)
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	entry, ok := c.Get("test/new-model")
	if !ok {
		t.Fatal("expected test/new-model after refresh")
	}
	if entry.ContextLength != 256000 {
		t.Errorf("ContextLength=%d, want 256000", entry.ContextLength)
	}
	if entry.InputPrice != 1.0 {
		t.Errorf("InputPrice=%f, want 1.0", entry.InputPrice)
	}

	// Verify KnownModels was updated
	meta, ok := KnownModels["test/new-model"]
	if !ok {
		t.Fatal("expected test/new-model in KnownModels")
	}
	if meta.ContextLength != 256000 {
		t.Errorf("KnownModels ContextLength=%d, want 256000", meta.ContextLength)
	}

	// Cleanup
	delete(KnownModels, "test/new-model")
}

func TestCatalogFallbackToCache(t *testing.T) {
	manifest := testManifest()
	data, _ := json.Marshal(manifest)

	cacheDir := t.TempDir()
	cachePath := filepath.Join(cacheDir, "model_catalog.json")
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := CatalogConfig{
		RemoteURL:    "http://localhost:1/nonexistent",
		CacheDir:     cacheDir,
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   &http.Client{Timeout: 100 * time.Millisecond},
	}

	c := NewCatalog(cfg)

	entry, ok := c.Get("test/new-model")
	if !ok {
		t.Fatal("expected test/new-model loaded from cache")
	}
	if entry.ContextLength != 256000 {
		t.Errorf("ContextLength=%d, want 256000", entry.ContextLength)
	}

	// Cleanup
	delete(KnownModels, "test/new-model")
}

func TestCatalogRefreshFailureDoesNotClear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := CatalogConfig{
		RemoteURL:    srv.URL,
		CacheDir:     t.TempDir(),
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   srv.Client(),
	}

	c := NewCatalog(cfg)
	initialLen := c.Len()

	err := c.Refresh()
	if err == nil {
		t.Fatal("expected error from 500 response")
	}

	if c.Len() != initialLen {
		t.Errorf("catalog size changed after failed refresh: got %d, want %d", c.Len(), initialLen)
	}
}

func TestCatalogEmptyManifestRejected(t *testing.T) {
	empty := &CatalogManifest{Version: "1.0", Models: []CatalogEntry{}}
	data, _ := json.Marshal(empty)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()

	cfg := CatalogConfig{
		RemoteURL:    srv.URL,
		CacheDir:     t.TempDir(),
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   srv.Client(),
	}

	c := NewCatalog(cfg)
	err := c.Refresh()
	if err == nil {
		t.Fatal("expected error for empty manifest")
	}
}

func TestCatalogList(t *testing.T) {
	cfg := CatalogConfig{CacheDir: t.TempDir()}
	c := NewCatalog(cfg)

	list := c.List()
	if len(list) == 0 {
		t.Fatal("expected non-empty list")
	}
	if len(list) != c.Len() {
		t.Errorf("List() returned %d items, Len() returned %d", len(list), c.Len())
	}
}

func TestCatalogSaveAndLoadCache(t *testing.T) {
	manifest := testManifest()
	data, _ := json.Marshal(manifest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	cfg := CatalogConfig{
		RemoteURL:    srv.URL,
		CacheDir:     cacheDir,
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   srv.Client(),
	}

	c := NewCatalog(cfg)
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Verify cache file created
	cachePath := filepath.Join(cacheDir, "model_catalog.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("expected cache file to exist")
	}

	// Create new catalog from same cache dir, no remote available
	cfg2 := CatalogConfig{
		RemoteURL:    "http://localhost:1/nonexistent",
		CacheDir:     cacheDir,
		RefreshEvery: 1 * time.Hour,
		HTTPClient:   &http.Client{Timeout: 100 * time.Millisecond},
	}
	c2 := NewCatalog(cfg2)

	entry, ok := c2.Get("test/new-model")
	if !ok {
		t.Fatal("expected test/new-model from cache in new catalog instance")
	}
	if entry.Provider != "test" {
		t.Errorf("Provider=%q, want %q", entry.Provider, "test")
	}

	// Cleanup
	delete(KnownModels, "test/new-model")
}

func TestCatalogStartStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := CatalogConfig{
		RemoteURL:    srv.URL,
		CacheDir:     t.TempDir(),
		RefreshEvery: 50 * time.Millisecond,
		HTTPClient:   srv.Client(),
	}

	c := NewCatalog(cfg)
	c.Start()
	time.Sleep(80 * time.Millisecond)
	c.Stop()
	// No panic or goroutine leak — test passes if it completes.
}
