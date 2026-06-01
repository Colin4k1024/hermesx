package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bundleRoot builds a temporary bundle root dir containing bundle.yaml and skill dirs.
func setupBundleRoot(t *testing.T, yaml string, skillNames []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write bundle.yaml: %v", err)
	}
	for _, name := range skillNames {
		mkSkillDir(t, dir, name)
	}
	return dir
}

// seedManifest writes a TenantManifest JSON for the given tenantID into the mock store.
func seedManifest(t *testing.T, store *mockObjectStore, tenantID string, m *TenantManifest) {
	t.Helper()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	store.objects[tenantID+manifestKey] = data
}

// readManifest deserialises the tenant manifest from the mock store.
func readManifest(t *testing.T, store *mockObjectStore, tenantID string) *TenantManifest {
	t.Helper()
	data, ok := store.objects[tenantID+manifestKey]
	if !ok {
		t.Fatalf("manifest not found for tenant %q", tenantID)
	}
	var m TenantManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return &m
}

// --- Tests ---------------------------------------------------------------

// TestSyncBundle_UploadsSkills verifies that a standard sync uploads all skill files
// and records them in the tenant manifest with the bundle source tag.
func TestSyncBundle_UploadsSkills(t *testing.T) {
	const tenantID = "tenant-alpha"
	bundleYAML := `
name: dev-bundle
version: "1.0.0"
skills:
  - name: code-review
    required: true
  - name: testing
    required: true
`
	root := setupBundleRoot(t, bundleYAML, []string{"code-review", "testing"})
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}

	manifest, err := LoadBundleManifest(filepath.Join(root, "bundle.yaml"))
	if err != nil {
		t.Fatalf("LoadBundleManifest: %v", err)
	}

	if err := prov.SyncBundle(context.Background(), tenantID, manifest, root); err != nil {
		t.Fatalf("SyncBundle: %v", err)
	}

	// Both skills must be in the OSS store.
	if _, ok := store.objects[tenantID+"/code-review/SKILL.md"]; !ok {
		t.Error("code-review/SKILL.md not uploaded")
	}
	if _, ok := store.objects[tenantID+"/testing/SKILL.md"]; !ok {
		t.Error("testing/SKILL.md not uploaded")
	}

	// Manifest must reflect the bundle source.
	m := readManifest(t, store, tenantID)
	src := "bundle:dev-bundle"
	if m.Skills["code-review"].Source != src {
		t.Errorf("code-review source: got %q, want %q", m.Skills["code-review"].Source, src)
	}
	if m.Skills["testing"].UserModified {
		t.Error("testing.UserModified should be false after bundle sync")
	}
}

// TestSyncBundle_RespectsUserModified verifies that a user-modified skill is NOT
// overwritten when AllowTenantOverride is true (the default protective mode).
func TestSyncBundle_RespectsUserModified(t *testing.T) {
	const tenantID = "tenant-beta"
	bundleYAML := `
name: ops-bundle
version: "1.0.0"
policy:
  allow_tenant_override: true
skills:
  - name: deploy
    required: false
`
	root := setupBundleRoot(t, bundleYAML, []string{"deploy"})
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}

	// Pre-populate with a user-modified version of the skill.
	originalContent := []byte("# user custom deploy skill")
	store.objects[tenantID+"/deploy/SKILL.md"] = originalContent
	seedManifest(t, store, tenantID, &TenantManifest{
		Version: 1,
		Skills: map[string]TenantSkillMeta{
			"deploy": {Source: "bundle:ops-bundle", UserModified: true, InstalledAt: time.Now()},
		},
	})

	manifest, _ := LoadBundleManifest(filepath.Join(root, "bundle.yaml"))
	if err := prov.SyncBundle(context.Background(), tenantID, manifest, root); err != nil {
		t.Fatalf("SyncBundle: %v", err)
	}

	// The user's content must still be intact.
	got := store.objects[tenantID+"/deploy/SKILL.md"]
	if string(got) != string(originalContent) {
		t.Errorf("user content overwritten; got %q, want %q", got, originalContent)
	}
}

// TestSyncBundle_ForceOverrideUserModified verifies that when AllowTenantOverride is
// false the bundle wins even over a user-modified skill.
func TestSyncBundle_ForceOverrideUserModified(t *testing.T) {
	const tenantID = "tenant-gamma"
	bundleYAML := `
name: strict-bundle
version: "1.0.0"
policy:
  allow_tenant_override: false
skills:
  - name: security-scan
    required: true
`
	root := setupBundleRoot(t, bundleYAML, []string{"security-scan"})
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}

	// Pre-populate with a user-modified skill.
	store.objects[tenantID+"/security-scan/SKILL.md"] = []byte("# old user version")
	seedManifest(t, store, tenantID, &TenantManifest{
		Version: 1,
		Skills: map[string]TenantSkillMeta{
			"security-scan": {Source: "bundle:strict-bundle", UserModified: true},
		},
	})

	manifest, _ := LoadBundleManifest(filepath.Join(root, "bundle.yaml"))
	if err := prov.SyncBundle(context.Background(), tenantID, manifest, root); err != nil {
		t.Fatalf("SyncBundle: %v", err)
	}

	// Bundle content should have replaced the user version.
	bundleContent, _ := os.ReadFile(filepath.Join(root, "security-scan", "SKILL.md"))
	got := store.objects[tenantID+"/security-scan/SKILL.md"]
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(bundleContent)) {
		t.Errorf("bundle content not applied; got %q", got)
	}
	// UserModified must be reset.
	m := readManifest(t, store, tenantID)
	if m.Skills["security-scan"].UserModified {
		t.Error("UserModified should be false after forced override")
	}
}

// TestSyncBundle_MissingRequiredSkillReturnsError returns an error when a required
// skill directory does not exist in the bundle root.
func TestSyncBundle_MissingRequiredSkillReturnsError(t *testing.T) {
	const tenantID = "tenant-delta"
	bundleYAML := `
name: incomplete-bundle
version: "1.0.0"
skills:
  - name: ghost-skill
    required: true
`
	root := setupBundleRoot(t, bundleYAML, nil) // no skill dirs
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}

	manifest, _ := LoadBundleManifest(filepath.Join(root, "bundle.yaml"))
	err := prov.SyncBundle(context.Background(), tenantID, manifest, root)
	if err == nil {
		t.Error("expected error for missing required skill, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-skill") {
		t.Errorf("error should mention skill name; got: %v", err)
	}
}

// TestSyncBundle_MissingOptionalSkillIsSkipped verifies that a missing optional
// skill does not cause an error.
func TestSyncBundle_MissingOptionalSkillIsSkipped(t *testing.T) {
	const tenantID = "tenant-epsilon"
	bundleYAML := `
name: partial-bundle
version: "1.0.0"
skills:
  - name: optional-feature
    required: false
`
	root := setupBundleRoot(t, bundleYAML, nil) // intentionally no skill dirs
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}

	manifest, _ := LoadBundleManifest(filepath.Join(root, "bundle.yaml"))
	if err := prov.SyncBundle(context.Background(), tenantID, manifest, root); err != nil {
		t.Errorf("unexpected error for missing optional skill: %v", err)
	}
}

// TestSyncBundle_InvalidTenantID rejects invalid tenant IDs.
func TestSyncBundle_InvalidTenantID(t *testing.T) {
	root := t.TempDir()
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}
	m := &BundleManifest{Name: "b", Skills: nil}
	if err := prov.SyncBundle(context.Background(), "", m, root); err == nil {
		t.Error("expected error for empty tenantID")
	}
}

// TestSyncBundle_NilManifestReturnsError rejects a nil manifest.
func TestSyncBundle_NilManifestReturnsError(t *testing.T) {
	root := t.TempDir()
	store := &mockObjectStore{objects: make(map[string][]byte)}
	prov := &Provisioner{minio: store, bundledDir: root}
	if err := prov.SyncBundle(context.Background(), "t1", nil, root); err == nil {
		t.Error("expected error for nil manifest")
	}
}
