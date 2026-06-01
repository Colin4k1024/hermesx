package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBundledPluginManifests_ValidNestedTree(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "platforms", "ntfy")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`
name: ntfy
description: notification platform
version: 1.0.0
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "adapter.go"), []byte("package ntfy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	issues, err := ValidateBundledPluginManifests(root)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestValidateBundledPluginManifests_MissingManifest(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "web", "xai")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "provider.go"), []byte("package xai\n"), 0644); err != nil {
		t.Fatal(err)
	}

	issues, err := ValidateBundledPluginManifests(root)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %#v", issues)
	}
	if issues[0].Path != pluginDir {
		t.Fatalf("issue path = %q, want %q", issues[0].Path, pluginDir)
	}
}

func TestValidateBundledPluginManifests_InvalidManifest(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "security-guidance")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("description: missing name\n"), 0644); err != nil {
		t.Fatal(err)
	}

	issues, err := ValidateBundledPluginManifests(root)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %#v", issues)
	}
	if issues[0].Message != "plugin manifest missing required name" {
		t.Fatalf("issue message = %q", issues[0].Message)
	}
}

func TestValidateBundledPluginManifests_MissingRootIsNoop(t *testing.T) {
	issues, err := ValidateBundledPluginManifests(filepath.Join(t.TempDir(), "plugins"))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}
