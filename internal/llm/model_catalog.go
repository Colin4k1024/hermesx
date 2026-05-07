package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CatalogEntry is a single model entry in the remote manifest.
type CatalogEntry struct {
	Name           string  `json:"name"`
	ShortName      string  `json:"short_name,omitempty"`
	Provider       string  `json:"provider"`
	ContextLength  int     `json:"context_length"`
	MaxOutput      int     `json:"max_output"`
	SupportsTools  bool    `json:"supports_tools"`
	SupportsVision bool    `json:"supports_vision"`
	Reasoning      bool    `json:"reasoning,omitempty"`
	InputPrice     float64 `json:"input_price,omitempty"`
	OutputPrice    float64 `json:"output_price,omitempty"`
}

// CatalogManifest is the JSON structure served by the remote endpoint.
type CatalogManifest struct {
	Version   string         `json:"version"`
	UpdatedAt string         `json:"updated_at"`
	Models    []CatalogEntry `json:"models"`
}

// CatalogConfig controls the remote model catalog behavior.
type CatalogConfig struct {
	RemoteURL    string
	RefreshEvery time.Duration
	CacheDir     string
	HTTPClient   *http.Client
}

// DefaultCatalogConfig returns production defaults.
func DefaultCatalogConfig(hermesHome string) CatalogConfig {
	return CatalogConfig{
		RemoteURL:    "https://hermes-agent.github.io/model-catalog/manifest.json",
		RefreshEvery: 1 * time.Hour,
		CacheDir:     filepath.Join(hermesHome, "cache"),
		HTTPClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Catalog manages model metadata with embedded defaults + remote override.
type Catalog struct {
	mu       sync.RWMutex
	entries  map[string]CatalogEntry
	config   CatalogConfig
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewCatalog creates a Catalog seeded from KnownModels.
func NewCatalog(cfg CatalogConfig) *Catalog {
	c := &Catalog{
		entries: make(map[string]CatalogEntry),
		config:  cfg,
		stopCh:  make(chan struct{}),
	}
	c.seedFromKnownModels()
	c.loadFromCache()
	return c
}

func (c *Catalog) seedFromKnownModels() {
	for name, meta := range KnownModels {
		c.entries[name] = CatalogEntry{
			Name:           name,
			Provider:       providerFromName(name),
			ContextLength:  meta.ContextLength,
			MaxOutput:      meta.MaxOutput,
			SupportsTools:  meta.SupportsTools,
			SupportsVision: meta.SupportsVision,
		}
	}
}

// Start begins periodic background refresh. Non-blocking.
func (c *Catalog) Start() {
	go c.refreshLoop()
}

// Stop halts the background refresh goroutine.
func (c *Catalog) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}

func (c *Catalog) refreshLoop() {
	if err := c.Refresh(); err != nil {
		slog.Warn("Initial catalog refresh failed, using cached/embedded defaults", "error", err)
	}

	ticker := time.NewTicker(c.config.RefreshEvery)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.Refresh(); err != nil {
				slog.Debug("Catalog refresh failed", "error", err)
			}
		}
	}
}

// Refresh fetches the remote manifest and updates the catalog.
func (c *Catalog) Refresh() error {
	manifest, err := c.fetchRemote()
	if err != nil {
		return err
	}
	c.applyManifest(manifest)
	c.saveToCache(manifest)
	slog.Info("Model catalog refreshed", "version", manifest.Version, "models", len(manifest.Models))
	return nil
}

func (c *Catalog) fetchRemote() (*CatalogManifest, error) {
	req, err := http.NewRequest("GET", c.config.RemoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: build request: %w", err)
	}
	req.Header.Set("User-Agent", "hermesx/2.0")

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog: remote returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("catalog: read body: %w", err)
	}

	var manifest CatalogManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("catalog: parse JSON: %w", err)
	}

	if len(manifest.Models) == 0 {
		return nil, fmt.Errorf("catalog: manifest contains no models")
	}

	return &manifest, nil
}

func (c *Catalog) applyManifest(m *CatalogManifest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range m.Models {
		if entry.Name == "" {
			continue
		}
		c.entries[entry.Name] = entry
	}

	// Update the global KnownModels map so GetModelMeta benefits.
	for _, entry := range m.Models {
		if entry.Name == "" {
			continue
		}
		KnownModels[entry.Name] = ModelMeta{
			ContextLength:  entry.ContextLength,
			MaxOutput:      entry.MaxOutput,
			SupportsTools:  entry.SupportsTools,
			SupportsVision: entry.SupportsVision,
		}
	}
}

func (c *Catalog) cacheFilePath() string {
	return filepath.Join(c.config.CacheDir, "model_catalog.json")
}

func (c *Catalog) saveToCache(m *CatalogManifest) {
	if c.config.CacheDir == "" {
		return
	}
	_ = os.MkdirAll(c.config.CacheDir, 0o755)
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.cacheFilePath(), data, 0o644)
}

func (c *Catalog) loadFromCache() {
	path := c.cacheFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var manifest CatalogManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return
	}
	if len(manifest.Models) > 0 {
		c.applyManifest(&manifest)
		slog.Debug("Loaded model catalog from cache", "models", len(manifest.Models))
	}
}

// Get returns a catalog entry by model name.
func (c *Catalog) Get(name string) (CatalogEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[name]
	return e, ok
}

// List returns all catalog entries.
func (c *Catalog) List() []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]CatalogEntry, 0, len(c.entries))
	for _, e := range c.entries {
		result = append(result, e)
	}
	return result
}

// Len returns the number of models in the catalog.
func (c *Catalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func providerFromName(name string) string {
	provider, _ := StripProviderPrefix(name)
	return provider
}
