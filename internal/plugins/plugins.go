// Package plugins implements plugin discovery and loading for Hermes.
// Plugins are discovered from user-level (~/.hermes/plugins/) and
// project-level (./hermes_plugins/) directories.
package plugins

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/config"
	"github.com/hermes-agent/hermes-agent-go/internal/tools"
	"gopkg.in/yaml.v3"
)

// Plugin represents a discovered plugin.
type Plugin struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Path        string `yaml:"-"` // filesystem path, not serialized
	Type        string `yaml:"-"` // "user", "project"
}

// PluginManifest is the structure of a plugin.yaml file.
type PluginManifest struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Version     string       `yaml:"version"`
	Author      string       `yaml:"author"`
	Tools       []PluginTool `yaml:"tools"`
}

// PluginTool describes a tool defined by a plugin.
type PluginTool struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Toolset     string         `yaml:"toolset"`
	Emoji       string         `yaml:"emoji"`
	Parameters  map[string]any `yaml:"parameters"`
	Command     string         `yaml:"command"` // shell command to execute
}

// DiscoverPlugins scans known plugin directories and returns all found plugins.
func DiscoverPlugins() []Plugin {
	var plugins []Plugin

	// 1. User plugins: ~/.hermes/plugins/
	userPluginsDir := filepath.Join(config.HermesHome(), "plugins")
	plugins = append(plugins, scanPluginDir(userPluginsDir, "user")...)

	// 2. Project plugins: ./hermes_plugins/
	cwd, err := os.Getwd()
	if err == nil {
		projectPluginsDir := filepath.Join(cwd, "hermes_plugins")
		plugins = append(plugins, scanPluginDir(projectPluginsDir, "project")...)
	}

	return plugins
}

// scanPluginDir scans a directory for plugin sub-directories containing plugin.yaml.
func scanPluginDir(dir, pluginType string) []Plugin {
	var plugins []Plugin

	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist — not an error.
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			slog.Debug("Skipping plugin dir (no plugin.yaml)", "dir", pluginDir)
			continue
		}

		var manifest PluginManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			slog.Warn("Invalid plugin.yaml", "path", manifestPath, "error", err)
			continue
		}

		name := manifest.Name
		if name == "" {
			name = entry.Name()
		}

		plugins = append(plugins, Plugin{
			Name:        name,
			Description: manifest.Description,
			Version:     manifest.Version,
			Path:        pluginDir,
			Type:        pluginType,
		})
	}

	return plugins
}

// LoadPlugin loads a plugin's tools into the global tool registry.
func LoadPlugin(plugin Plugin) error {
	manifestPath := filepath.Join(plugin.Path, "plugin.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read plugin manifest: %w", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse plugin manifest: %w", err)
	}

	for _, pt := range manifest.Tools {
		if err := registerPluginTool(plugin, pt); err != nil {
			slog.Warn("Failed to register plugin tool",
				"plugin", plugin.Name, "tool", pt.Name, "error", err)
			continue
		}
		slog.Info("Registered plugin tool", "plugin", plugin.Name, "tool", pt.Name)
	}

	return nil
}

// LoadAllPlugins discovers and loads all plugins.
func LoadAllPlugins() ([]Plugin, error) {
	plugins := DiscoverPlugins()

	var loadErrors []error
	for _, p := range plugins {
		if err := LoadPlugin(p); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("plugin %s: %w", p.Name, err))
		}
	}

	if len(loadErrors) > 0 {
		return plugins, fmt.Errorf("some plugins failed to load (%d errors)", len(loadErrors))
	}

	return plugins, nil
}

// registerPluginTool creates a tool entry from a plugin tool definition and
// registers it with the global tool registry.
func registerPluginTool(plugin Plugin, pt PluginTool) error {
	if pt.Name == "" {
		return fmt.Errorf("tool has no name")
	}

	toolset := pt.Toolset
	if toolset == "" {
		toolset = "plugin_" + plugin.Name
	}

	description := pt.Description
	if description == "" {
		description = fmt.Sprintf("Tool from plugin %s", plugin.Name)
	}

	schema := map[string]any{
		"description": description,
		"parameters": map[string]any{
			"type":       "object",
			"properties": pt.Parameters,
		},
	}

	// Build a handler that executes the plugin's command.
	command := pt.Command
	pluginPath := plugin.Path

	handler := func(args map[string]any, ctx *tools.ToolContext) string {
		if command == "" {
			return `{"error":"plugin tool has no command defined"}`
		}
		return executePluginCommand(command, pluginPath, args)
	}

	tools.Register(&tools.ToolEntry{
		Name:        pt.Name,
		Toolset:     toolset,
		Schema:      schema,
		Handler:     handler,
		Description: description,
		Emoji:       pt.Emoji,
	})

	return nil
}

// executePluginCommand runs a plugin's shell command and returns the output.
func executePluginCommand(command, pluginDir string, args map[string]any) string {
	env := os.Environ()
	for k, v := range args {
		// Sanitize key: only allow alphanumeric and underscore.
		safeKey := sanitizeEnvKey(k)
		if safeKey != "" {
			env = append(env, fmt.Sprintf("HERMES_ARG_%s=%v", safeKey, v))
		}
	}
	env = append(env, "HERMES_PLUGIN_DIR="+pluginDir)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = pluginDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Sprintf(`{"error":%q,"stderr":%q}`,
			fmt.Sprintf("plugin command failed: %v", err),
			truncateStr(stderr.String(), 2000))
	}

	output := stdout.String()
	if len(output) > 8000 {
		output = output[:8000] + "\n... (truncated)"
	}

	return fmt.Sprintf(`{"output":%q}`, output)
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// sanitizeEnvKey strips non-alphanumeric/underscore characters from env var keys.
func sanitizeEnvKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
