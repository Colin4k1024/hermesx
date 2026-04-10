package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPlugins_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	plugins := DiscoverPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverPlugins_WithManifest(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	pluginDir := filepath.Join(tmpDir, "plugins", "test-plugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`
name: test-plugin
description: A test plugin
version: 1.0.0
tools:
  - name: test_tool
    description: does nothing
    command: echo hello
`), 0644)

	plugins := DiscoverPlugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got '%s'", plugins[0].Name)
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/my-plugin.git", "my-plugin"},
		{"https://github.com/user/my-plugin", "my-plugin"},
		{"git@github.com:user/my-plugin.git", "my-plugin"},
		{"my-plugin", "my-plugin"},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			got := extractRepoName(tc.url)
			if got != tc.want {
				t.Errorf("extractRepoName(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestEnableDisablePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	// Initially not disabled.
	if IsPluginDisabled("my-plugin") {
		t.Error("expected plugin to not be disabled initially")
	}

	// Disable.
	if err := DisablePlugin("my-plugin"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !IsPluginDisabled("my-plugin") {
		t.Error("expected plugin to be disabled after DisablePlugin")
	}

	// Double disable is no-op.
	if err := DisablePlugin("my-plugin"); err != nil {
		t.Fatalf("double disable: %v", err)
	}

	// Enable.
	if err := EnablePlugin("my-plugin"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if IsPluginDisabled("my-plugin") {
		t.Error("expected plugin to be enabled after EnablePlugin")
	}
}

func TestUninstallPlugin_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	err := UninstallPlugin("nonexistent")
	if err == nil {
		t.Error("expected error when uninstalling nonexistent plugin")
	}
}

func TestReadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "plugin.yaml"), []byte(`
name: manifest-test
description: testing manifest read
version: 2.0.0
author: tester
tools:
  - name: greet
    description: says hello
    command: echo hello
`), 0644)

	manifest, err := ReadManifest(tmpDir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest.Name != "manifest-test" {
		t.Errorf("name = %q, want 'manifest-test'", manifest.Name)
	}
	if manifest.Version != "2.0.0" {
		t.Errorf("version = %q, want '2.0.0'", manifest.Version)
	}
	if len(manifest.Tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(manifest.Tools))
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than max", 10, "this is lo..."},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := truncateStr(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
		})
	}
}

func TestIsValidPluginName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"my-plugin", true},
		{"plugin_v2", true},
		{"cool.plugin", true},
		{"..", false},
		{".", false},
		{"", false},
		{"../../etc", false},
		{"a/../b", false},
		{"a/b", false},
		{"a\\b", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidPluginName(tc.name)
			if got != tc.want {
				t.Errorf("isValidPluginName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestSanitizeEnvKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal_key", "normal_key"},
		{"key=value", "keyvalue"},
		{"key\nnewline", "keynewline"},
		{"UPPER_123", "UPPER_123"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeEnvKey(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeEnvKey(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
