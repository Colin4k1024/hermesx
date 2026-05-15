package config

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectConfigDir is the directory name used for project-scoped configuration.
const ProjectConfigDir = ".hermes"

// projectConfigAllowedFields defines which config fields can be set at project level.
// Sensitive fields (api_key, database, redis, objstore) are NOT allowed in project config
// to prevent credentials from leaking into version-controlled repositories.
var projectConfigAllowedFields = map[string]bool{
	"model":          true,
	"max_iterations": true,
	"max_tokens":     true,
	"tool_delay":     true,
	"api_mode":       true,
	"display":        true,
	"terminal":       true,
	"memory":         true,
	"toolsets":       true,
	"reasoning":      true,
	"delegation":     true,
	"auxiliary":      true,
	"plugins":        true,
	"cache":          true,
}

// FindProjectRoot walks up from the given directory to find the nearest git root
// or a directory containing .hermes/config.yaml.
func FindProjectRoot(startDir string) string {
	// Try git rev-parse first (fast and reliable).
	if root := gitRoot(startDir); root != "" {
		return root
	}

	// Walk up looking for .hermes/config.yaml.
	dir := startDir
	for {
		candidate := filepath.Join(dir, ProjectConfigDir, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// gitRoot returns the git repository root for the given directory, or empty string.
func gitRoot(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LoadProjectConfig loads project-scoped configuration from {projectRoot}/.hermes/config.yaml.
// It only merges allowed fields (no credentials or infrastructure settings).
// Returns nil if no project config is found.
func LoadProjectConfig() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	root := FindProjectRoot(cwd)
	if root == "" {
		return nil
	}

	configPath := filepath.Join(root, ProjectConfigDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var projectCfg Config
	if err := yaml.Unmarshal(data, &projectCfg); err != nil {
		slog.Warn("Failed to parse project config", "path", configPath, "error", err)
		return nil
	}

	// Enforce security: clear sensitive fields that are not allowed at project level.
	sanitizeProjectConfig(&projectCfg)

	slog.Debug("Loaded project config", "path", configPath)
	return &projectCfg
}

// sanitizeProjectConfig clears sensitive fields from a project-scoped config.
func sanitizeProjectConfig(cfg *Config) {
	// Never allow credentials at project level.
	cfg.APIKey = ""
	cfg.Database = DatabaseConfig{}
	cfg.Redis = RedisConfig{}
	cfg.ObjStore = ObjStoreConfig{}
	cfg.Provider = "" // provider implies endpoint routing, keep it user-level
	cfg.BaseURL = ""  // base_url implies endpoint, keep it user-level
}
