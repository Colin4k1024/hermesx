package plugins

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/config"
	"gopkg.in/yaml.v3"
)

// EnablePlugin marks a plugin as enabled in the config.
func EnablePlugin(name string) error {
	cfg := config.Load()
	for _, p := range cfg.Plugins.Disabled {
		if p == name {
			// Remove from disabled list.
			var newDisabled []string
			for _, d := range cfg.Plugins.Disabled {
				if d != name {
					newDisabled = append(newDisabled, d)
				}
			}
			cfg.Plugins.Disabled = newDisabled
			return config.Save(cfg)
		}
	}
	return nil // already enabled (not in disabled list)
}

// DisablePlugin marks a plugin as disabled in the config.
func DisablePlugin(name string) error {
	cfg := config.Load()
	for _, p := range cfg.Plugins.Disabled {
		if p == name {
			return nil // already disabled
		}
	}
	cfg.Plugins.Disabled = append(cfg.Plugins.Disabled, name)
	return config.Save(cfg)
}

// IsPluginDisabled checks if a plugin is in the disabled list.
func IsPluginDisabled(name string) bool {
	cfg := config.Load()
	for _, p := range cfg.Plugins.Disabled {
		if p == name {
			return true
		}
	}
	return false
}

// InstallFromGit clones a plugin from a Git repository into the user plugins directory.
func InstallFromGit(repoURL string) (string, error) {
	pluginsDir := filepath.Join(config.HermesHome(), "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("create plugins dir: %w", err)
	}

	// Extract name from repo URL.
	name := extractRepoName(repoURL)
	if name == "" {
		return "", fmt.Errorf("cannot determine plugin name from URL: %s", repoURL)
	}

	destDir := filepath.Join(pluginsDir, name)
	if _, err := os.Stat(destDir); err == nil {
		return "", fmt.Errorf("plugin '%s' already installed at %s", name, destDir)
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, destDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	// Verify plugin.yaml exists.
	manifestPath := filepath.Join(destDir, "plugin.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Clean up invalid plugin.
		os.RemoveAll(destDir)
		return "", fmt.Errorf("cloned repo does not contain plugin.yaml")
	}

	slog.Info("Plugin installed", "name", name, "path", destDir)
	return name, nil
}

// UninstallPlugin removes a plugin directory.
func UninstallPlugin(name string) error {
	if !isValidPluginName(name) {
		return fmt.Errorf("invalid plugin name: %q", name)
	}

	pluginsDir := filepath.Join(config.HermesHome(), "plugins")
	pluginDir := filepath.Join(pluginsDir, name)

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}

	slog.Info("Plugin uninstalled", "name", name)
	return nil
}

// ListPluginsWithStatus returns all discovered plugins with their enabled/disabled status.
func ListPluginsWithStatus() []PluginStatus {
	plugins := DiscoverPlugins()
	var result []PluginStatus
	for _, p := range plugins {
		result = append(result, PluginStatus{
			Plugin:  p,
			Enabled: !IsPluginDisabled(p.Name),
		})
	}
	return result
}

// PluginStatus wraps a Plugin with its enabled state.
type PluginStatus struct {
	Plugin  Plugin
	Enabled bool
}

// PluginsConfig holds plugin configuration.
type PluginsConfig struct {
	Disabled []string `yaml:"disabled"`
}

func extractRepoName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	name := parts[len(parts)-1]
	// Sanitize to prevent path traversal.
	if !isValidPluginName(name) {
		return ""
	}
	return name
}

// isValidPluginName rejects names that could cause path traversal.
func isValidPluginName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, c := range name {
		if c == '/' || c == '\\' || c == '\x00' {
			return false
		}
	}
	if strings.Contains(name, "..") {
		return false
	}
	return true
}

// LoadEnabledPlugins discovers and loads only enabled plugins.
func LoadEnabledPlugins() ([]Plugin, error) {
	plugins := DiscoverPlugins()

	var loaded []Plugin
	var loadErrors []error
	for _, p := range plugins {
		if IsPluginDisabled(p.Name) {
			slog.Debug("Skipping disabled plugin", "name", p.Name)
			continue
		}
		if err := LoadPlugin(p); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("plugin %s: %w", p.Name, err))
			continue
		}
		loaded = append(loaded, p)
	}

	if len(loadErrors) > 0 {
		return loaded, fmt.Errorf("some plugins failed to load (%d errors)", len(loadErrors))
	}
	return loaded, nil
}

// ReadManifest reads and parses a plugin's manifest file.
func ReadManifest(pluginDir string) (*PluginManifest, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &manifest, nil
}
