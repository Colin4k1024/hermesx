package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeProjectConfig(t *testing.T) {
	cfg := &Config{
		Model:    "opus",
		APIKey:   "sk-secret-key",
		BaseURL:  "https://api.evil.com",
		Provider: "openai",
		Database: DatabaseConfig{URL: "postgres://localhost/db"},
		Redis:    RedisConfig{URL: "redis://localhost:6379"},
		Reasoning: ReasoningConfig{
			Enabled: true,
			Effort:  "high",
		},
		MaxIterations: 50,
	}

	sanitizeProjectConfig(cfg)

	// Sensitive fields should be cleared.
	if cfg.APIKey != "" {
		t.Errorf("APIKey should be cleared, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "" {
		t.Errorf("BaseURL should be cleared, got %q", cfg.BaseURL)
	}
	if cfg.Provider != "" {
		t.Errorf("Provider should be cleared, got %q", cfg.Provider)
	}
	if cfg.Database.URL != "" {
		t.Errorf("Database.URL should be cleared, got %q", cfg.Database.URL)
	}
	if cfg.Redis.URL != "" {
		t.Errorf("Redis.URL should be cleared, got %q", cfg.Redis.URL)
	}

	// Allowed fields should remain.
	if cfg.Model != "opus" {
		t.Errorf("Model should remain 'opus', got %q", cfg.Model)
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("MaxIterations should remain 50, got %d", cfg.MaxIterations)
	}
	if cfg.Reasoning.Effort != "high" {
		t.Errorf("Reasoning.Effort should remain 'high', got %q", cfg.Reasoning.Effort)
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Create a temp dir with .hermes/config.yaml
	tmpDir := t.TempDir()
	hermesDir := filepath.Join(tmpDir, ".hermes")
	if err := os.MkdirAll(hermesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte("model: opus\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested subdir
	subDir := filepath.Join(tmpDir, "src", "internal")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// FindProjectRoot from subdir should find the temp root
	root := FindProjectRoot(subDir)
	if root != tmpDir {
		t.Errorf("FindProjectRoot(%q) = %q, want %q", subDir, root, tmpDir)
	}

	// FindProjectRoot from a dir without .hermes should return "" (no git either)
	emptyDir := t.TempDir()
	root = FindProjectRoot(emptyDir)
	if root == emptyDir {
		t.Errorf("FindProjectRoot(%q) should not find root in empty dir", emptyDir)
	}
}

func TestLoadProjectConfig_MergesCorrectly(t *testing.T) {
	// Save current dir and restore after test.
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create a project with .hermes/config.yaml
	tmpDir := t.TempDir()
	hermesDir := filepath.Join(tmpDir, ".hermes")
	os.MkdirAll(hermesDir, 0755)
	os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte(`
model: haiku
max_iterations: 20
reasoning:
  enabled: true
  effort: high
api_key: should-be-stripped
`), 0644)

	// Change to the project dir
	os.Chdir(tmpDir)

	cfg := LoadProjectConfig()
	if cfg == nil {
		t.Fatal("LoadProjectConfig() returned nil")
	}

	if cfg.Model != "haiku" {
		t.Errorf("Model = %q, want 'haiku'", cfg.Model)
	}
	if cfg.MaxIterations != 20 {
		t.Errorf("MaxIterations = %d, want 20", cfg.MaxIterations)
	}
	if cfg.Reasoning.Effort != "high" {
		t.Errorf("Reasoning.Effort = %q, want 'high'", cfg.Reasoning.Effort)
	}
	// APIKey should be stripped
	if cfg.APIKey != "" {
		t.Errorf("APIKey should be stripped, got %q", cfg.APIKey)
	}
}
