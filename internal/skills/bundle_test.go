package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// writeBundleYAML writes content to <dir>/bundle.yaml and returns dir.
func writeBundleYAML(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write bundle.yaml: %v", err)
	}
	return dir
}

// mkSkillDir creates <dir>/<name>/SKILL.md with minimal content.
func mkSkillDir(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("# "+name), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

// TestValidateBundleManifest_MissingRoot shows that a non-existent root is a no-op.
func TestValidateBundleManifest_MissingRoot(t *testing.T) {
	issues := ValidateBundleManifest("/nonexistent/bundle/root")
	if len(issues) != 0 {
		t.Errorf("expected no issues for missing root, got %v", issues)
	}
}

// TestValidateBundleManifest_EmptyRoot shows that an empty root is a no-op.
func TestValidateBundleManifest_EmptyRoot(t *testing.T) {
	issues := ValidateBundleManifest("")
	if len(issues) != 0 {
		t.Errorf("expected no issues for empty root, got %v", issues)
	}
}

// TestValidateBundleManifest_NoBundleFile reports an issue when bundle.yaml is absent.
func TestValidateBundleManifest_NoBundleFile(t *testing.T) {
	dir := t.TempDir()
	issues := ValidateBundleManifest(dir)
	if len(issues) != 1 || issues[0].Kind != BundleIssueNoBundleFile {
		t.Errorf("expected one BundleIssueNoBundleFile, got %v", issues)
	}
}

// TestValidateBundleManifest_InvalidYAML reports an issue on malformed YAML.
func TestValidateBundleManifest_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeBundleYAML(t, dir, "name: [\ninvalid")
	issues := ValidateBundleManifest(dir)
	if len(issues) != 1 || issues[0].Kind != BundleIssueInvalidYAML {
		t.Errorf("expected one BundleIssueInvalidYAML, got %v", issues)
	}
}

// TestValidateBundleManifest_MissingName reports an issue when name is absent.
func TestValidateBundleManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeBundleYAML(t, dir, "version: \"1.0.0\"\nskills: []\n")
	issues := ValidateBundleManifest(dir)
	if len(issues) != 1 || issues[0].Kind != BundleIssueMissingName {
		t.Errorf("expected one BundleIssueMissingName, got %v", issues)
	}
}

// TestValidateBundleManifest_MissingRequiredSkill reports a missing required skill dir.
func TestValidateBundleManifest_MissingRequiredSkill(t *testing.T) {
	dir := t.TempDir()
	writeBundleYAML(t, dir, `
name: my-bundle
version: "1.0.0"
skills:
  - name: code-review
    required: true
  - name: testing
    required: false
`)
	// "testing" dir exists; "code-review" does not.
	mkSkillDir(t, dir, "testing")
	issues := ValidateBundleManifest(dir)
	if len(issues) != 1 || issues[0].Kind != BundleIssueMissingRequiredSkill || issues[0].Ref != "code-review" {
		t.Errorf("expected one BundleIssueMissingRequiredSkill for code-review, got %v", issues)
	}
}

// TestValidateBundleManifest_Valid passes with a well-formed bundle.
func TestValidateBundleManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	writeBundleYAML(t, dir, `
name: dev-essentials
version: "2.0.0"
description: Essential developer skills.
author: hermesx
policy:
  allow_tenant_override: true
  allow_user_modification: true
skills:
  - name: code-review
    required: true
  - name: testing
    required: true
`)
	mkSkillDir(t, dir, "code-review")
	mkSkillDir(t, dir, "testing")
	issues := ValidateBundleManifest(dir)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid bundle, got %v", issues)
	}
}

// TestLoadBundleManifest parses a valid bundle.yaml and checks fields.
func TestLoadBundleManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	writeBundleYAML(t, dir, `
name: ops-bundle
version: "0.1.0"
description: Ops skills.
policy:
  allow_tenant_override: false
  allow_user_modification: true
skills:
  - name: deployment
    required: true
    min_version: "0.2.0"
  - name: monitoring
    required: false
`)
	m, err := LoadBundleManifest(filepath.Join(dir, "bundle.yaml"))
	if err != nil {
		t.Fatalf("LoadBundleManifest: %v", err)
	}
	if m.Name != "ops-bundle" {
		t.Errorf("name: got %q, want %q", m.Name, "ops-bundle")
	}
	if len(m.Skills) != 2 {
		t.Errorf("skills count: got %d, want 2", len(m.Skills))
	}
	if m.Policy.AllowTenantOverride {
		t.Error("allow_tenant_override should be false")
	}
	if m.Skills[0].MinVersion != "0.2.0" {
		t.Errorf("min_version: got %q, want %q", m.Skills[0].MinVersion, "0.2.0")
	}
}

// TestLoadBundleManifest_Missing returns an error for a non-existent file.
func TestLoadBundleManifest_Missing(t *testing.T) {
	_, err := LoadBundleManifest("/nonexistent/bundle.yaml")
	if err == nil {
		t.Error("expected error for missing bundle.yaml, got nil")
	}
}
