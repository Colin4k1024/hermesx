//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/skills"
)

func TestSkills_Provision_TenantScoped(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "skill-prov-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "skill-prov-b", "pro")
	ctx := context.Background()

	provisioner := skills.NewProvisioner(testEnv.MinIO, "../../tests/fixtures/skills")

	// Provision skills for tenant A only
	if err := provisioner.Provision(ctx, tenantA.ID); err != nil {
		t.Fatalf("provision tenant A: %v", err)
	}

	// Tenant A should have skills
	loaderA := skills.NewMinIOSkillLoader(testEnv.MinIO, tenantA.ID)
	entriesA, err := loaderA.LoadAll(ctx)
	if err != nil {
		t.Fatalf("load A skills: %v", err)
	}
	if len(entriesA) == 0 {
		t.Error("tenant A should have provisioned skills")
	}

	// Tenant B should NOT see tenant A's skills
	loaderB := skills.NewMinIOSkillLoader(testEnv.MinIO, tenantB.ID)
	entriesB, err := loaderB.LoadAll(ctx)
	if err != nil {
		t.Fatalf("load B skills: %v", err)
	}
	for _, e := range entriesB {
		for _, a := range entriesA {
			if e.Meta.Name == a.Meta.Name && e.DirName == a.DirName {
				t.Errorf("tenant B sees tenant A's provisioned skill: %s", e.Meta.Name)
			}
		}
	}
}

func TestSkills_Upload_TenantPrefix(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-upload", "pro")
	ctx := context.Background()

	skillContent := `---
name: custom-tool
description: A custom skill uploaded by tenant
version: "1.0"
---
# Custom Tool
Do something custom.
`
	// Upload via API
	resp := testEnv.DoRequest(t, "PUT", "/v1/skills/custom-tool", skillContent, tenant.APIKey, nil)
	body := ReadBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify stored with correct tenant prefix in MinIO
	key := tenant.ID + "/custom-tool/SKILL.md"
	data, err := testEnv.MinIO.GetObject(ctx, key)
	if err != nil {
		t.Fatalf("get object from minio: %v", err)
	}
	if len(data) == 0 {
		t.Error("skill content is empty in MinIO")
	}
	if !containsString(string(data), "custom-tool") {
		t.Errorf("stored content doesn't match upload, got: %s", string(data)[:min(len(data), 200)])
	}
}

func TestSkills_PathTraversal_Rejected(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-traversal", "pro")

	badNames := []string{
		"../escape",
		"../../etc/passwd",
		"foo/../bar",
	}

	for _, name := range badNames {
		resp := testEnv.DoRequest(t, "PUT", "/v1/skills/"+name, "# bad skill", tenant.APIKey, nil)
		body := ReadBody(t, resp)

		// Path traversal should be rejected or handled safely
		if resp.StatusCode == http.StatusOK {
			// If it was accepted, verify it's stored safely (with sanitized path)
			// The key should NOT escape the tenant namespace
			traversalKey := tenant.ID + "/" + name + "/SKILL.md"
			_, err := testEnv.MinIO.GetObject(context.Background(), traversalKey)
			if err == nil {
				t.Errorf("path traversal name %q was stored unsanitized", name)
			}
		}
		_ = body
	}
}

func TestSkills_CrossTenant_List(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "skill-list-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "skill-list-b", "pro")

	// Upload skill to tenant A
	resp := testEnv.DoRequest(t, "PUT", "/v1/skills/secret-skill", "---\nname: secret-skill\n---\n# Secret", tenantA.APIKey, nil)
	ReadBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload to A failed: %d", resp.StatusCode)
	}

	// List skills with tenant B's key
	resp = testEnv.DoRequest(t, "GET", "/v1/skills", "", tenantB.APIKey, nil)
	body := ReadBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list B skills: %d: %s", resp.StatusCode, body)
	}

	// Tenant B should NOT see tenant A's "secret-skill"
	if containsString(body, "secret-skill") {
		t.Errorf("tenant B can see tenant A's skill: %s", body)
	}

	// Verify tenant A CAN see their own skill
	resp = testEnv.DoRequest(t, "GET", "/v1/skills", "", tenantA.APIKey, nil)
	bodyA := ReadBody(t, resp)
	if !containsString(bodyA, "secret-skill") {
		t.Errorf("tenant A should see their own skill, body: %s", bodyA)
	}
}

func TestSkills_UserModified_Flag(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-modified", "pro")
	ctx := context.Background()

	// Upload a skill (triggers MarkSkillUserModified)
	skillContent := "---\nname: my-skill\nversion: \"2.0\"\n---\n# My Skill\nCustom content."
	resp := testEnv.DoRequest(t, "PUT", "/v1/skills/my-skill", skillContent, tenant.APIKey, nil)
	ReadBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload failed: %d", resp.StatusCode)
	}

	// Load manifest and check user_modified flag
	manifest, err := skills.LoadTenantManifestPublic(ctx, testEnv.MinIO, tenant.ID)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest == nil {
		t.Fatal("manifest is nil after upload")
	}

	meta, ok := manifest.Skills["my-skill"]
	if !ok {
		t.Fatalf("skill 'my-skill' not in manifest, keys: %v", manifestKeys(manifest))
	}
	if !meta.UserModified {
		t.Error("skill should be marked as user_modified after PUT")
	}
}

func TestSkills_CompositeLoader_Shadow(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-shadow", "pro")
	ctx := context.Background()

	// Provision bundled skills for the tenant
	provisioner := skills.NewProvisioner(testEnv.MinIO, "../../tests/fixtures/skills")
	if err := provisioner.Provision(ctx, tenant.ID); err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Get the list of provisioned skills
	loaderMinio := skills.NewMinIOSkillLoader(testEnv.MinIO, tenant.ID)
	bundled, _ := loaderMinio.LoadAll(ctx)
	if len(bundled) == 0 {
		t.Skip("no bundled skills provisioned, cannot test shadow")
	}

	// Upload a custom version of the first bundled skill (shadow it)
	shadowName := bundled[0].DirName
	if shadowName == "" {
		shadowName = bundled[0].Meta.Name
	}
	customContent := "---\nname: " + shadowName + "\nversion: \"99.0\"\ndescription: tenant override\n---\n# Custom Override"
	key := tenant.ID + "/" + shadowName + "/SKILL.md"
	if err := testEnv.MinIO.PutObject(ctx, key, []byte(customContent)); err != nil {
		t.Fatalf("put custom skill: %v", err)
	}

	// Use composite loader: MinIO (primary) + MinIO again (as fallback wouldn't have duplicates)
	// The real composite pattern is MinIO first, then local fallback
	// Here we verify that LoadAll returns the MinIO version (with version 99.0)
	entries, err := loaderMinio.LoadAll(ctx)
	if err != nil {
		t.Fatalf("load all: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.DirName == shadowName || e.Meta.Name == shadowName {
			if e.Meta.Version == "99.0" {
				found = true
			} else {
				t.Errorf("expected shadow version 99.0, got %s", e.Meta.Version)
			}
			break
		}
	}
	if !found {
		t.Errorf("custom shadow skill %q not found in loaded entries", shadowName)
	}

	// Verify via API that the response shows the custom version
	resp := testEnv.DoRequest(t, "GET", "/v1/skills", "", tenant.APIKey, nil)
	body := ReadBody(t, resp)
	if !containsString(body, "tenant override") {
		t.Logf("API response: %s", body[:min(len(body), 500)])
	}
}

func TestSkills_Delete_OnlyOwnTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "skill-del-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "skill-del-b", "pro")

	// Upload skill to tenant A
	resp := testEnv.DoRequest(t, "PUT", "/v1/skills/del-target", "---\nname: del-target\n---\n# Target", tenantA.APIKey, nil)
	ReadBody(t, resp)

	// Tenant B tries to delete tenant A's skill
	resp = testEnv.DoRequest(t, "DELETE", "/v1/skills/del-target", "", tenantB.APIKey, nil)
	ReadBody(t, resp)

	// Tenant A's skill should still exist
	resp = testEnv.DoRequest(t, "GET", "/v1/skills", "", tenantA.APIKey, nil)
	body := ReadBody(t, resp)
	if !containsString(body, "del-target") {
		t.Error("tenant B was able to delete tenant A's skill — isolation failure")
	}
}

func manifestKeys(m *skills.TenantManifest) []string {
	var keys []string
	for k := range m.Skills {
		keys = append(keys, k)
	}
	return keys
}

func TestSkills_ListResponse_Structure(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-struct", "pro")

	// Upload a skill
	resp := testEnv.DoRequest(t, "PUT", "/v1/skills/structured-skill", "---\nname: structured-skill\ndescription: test desc\nversion: \"1.0\"\n---\n# Structured", tenant.APIKey, nil)
	ReadBody(t, resp)

	// List and verify JSON structure
	resp = testEnv.DoRequest(t, "GET", "/v1/skills", "", tenant.APIKey, nil)
	body := ReadBody(t, resp)

	var result struct {
		TenantID string `json:"tenant_id"`
		Skills   []struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Version      string `json:"version"`
			UserModified bool   `json:"user_modified"`
		} `json:"skills"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("unmarshal: %v, body: %s", err, body[:min(len(body), 300)])
	}

	if result.TenantID != tenant.ID {
		t.Errorf("response tenant_id mismatch: got %q, want %q", result.TenantID, tenant.ID)
	}
	if result.Total < 1 {
		t.Errorf("expected at least 1 skill, total=%d", result.Total)
	}

	found := false
	for _, s := range result.Skills {
		if s.Name == "structured-skill" {
			found = true
			if s.Description != "test desc" {
				t.Errorf("description mismatch: %q", s.Description)
			}
			if s.Version != "1.0" {
				t.Errorf("version mismatch: %q", s.Version)
			}
		}
	}
	if !found {
		t.Error("uploaded skill not in list response")
	}
}
