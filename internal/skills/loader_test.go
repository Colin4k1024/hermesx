package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAllSkillsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0755)

	skills, err := LoadAllSkills()
	if err != nil {
		t.Fatalf("LoadAllSkills failed: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("Expected 0 skills in empty dir, got %d", len(skills))
	}
}

func TestLoadAllSkillsWithSkill(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillsDir := filepath.Join(tmpDir, "skills", "test-skill")
	os.MkdirAll(skillsDir, 0755)

	skillContent := `---
name: test-skill
description: A test skill
version: 1.0.0
---

# Test Skill
Instructions here.
`
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644)

	skills, err := LoadAllSkills()
	if err != nil {
		t.Fatalf("LoadAllSkills failed: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(skills))
	}
}

func TestBuildSkillsIndex(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillsDir := filepath.Join(tmpDir, "skills", "my-skill")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(`---
name: my-skill
description: Does things
---
# My Skill
`), 0644)

	skills, _ := LoadAllSkills()
	index := BuildSkillsIndex(skills)
	if len(index) != 1 {
		t.Errorf("Expected 1 skill in index, got %d", len(index))
	}
}

func TestFindSkill(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillsDir := filepath.Join(tmpDir, "skills", "finder-test")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(`---
name: finder-test
description: Test for FindSkill
---
# Finder Test
`), 0644)

	entry, err := FindSkill("finder-test")
	if err != nil {
		t.Fatalf("FindSkill failed: %v", err)
	}
	if entry == nil {
		t.Error("Expected to find skill 'finder-test'")
	}

	entry, _ = FindSkill("nonexistent-skill")
	if entry != nil {
		t.Error("Expected nil for nonexistent skill")
	}
}

func TestBuildSkillsPrompt(t *testing.T) {
	skills := []*SkillEntry{
		{Meta: &SkillMeta{Name: "test-skill", Description: "A test skill"}},
		{Meta: &SkillMeta{Name: "another-skill", Description: "Another skill"}},
	}

	prompt := BuildSkillsPrompt(skills)
	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}
	if !strings.Contains(prompt, "/test-skill") {
		t.Error("Expected '/test-skill' in prompt")
	}
	if !strings.Contains(prompt, "A test skill") {
		t.Error("Expected description in prompt")
	}
}

func TestBuildSkillsPrompt_Empty(t *testing.T) {
	prompt := BuildSkillsPrompt(nil)
	if prompt != "" {
		t.Error("Expected empty prompt for nil skills")
	}
}

func TestBuildSkillsPrompt_NoDescription(t *testing.T) {
	skills := []*SkillEntry{
		{Meta: &SkillMeta{Name: "nodesc"}},
	}

	prompt := BuildSkillsPrompt(skills)
	if !strings.Contains(prompt, "No description") {
		t.Error("Expected 'No description' fallback")
	}
}

func TestGetSkillsByCategory(t *testing.T) {
	skills := []*SkillEntry{
		{Meta: &SkillMeta{Name: "s1", Category: "tools"}},
		{Meta: &SkillMeta{Name: "s2", Category: "tools"}},
		{Meta: &SkillMeta{Name: "s3", Category: "workflow"}},
		{Meta: &SkillMeta{Name: "s4"}}, // no category -> "general"
	}

	groups := GetSkillsByCategory(skills)
	if len(groups["tools"]) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(groups["tools"]))
	}
	if len(groups["workflow"]) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(groups["workflow"]))
	}
	if len(groups["general"]) != 1 {
		t.Errorf("Expected 1 general, got %d", len(groups["general"]))
	}
}

func TestExcludedDirs(t *testing.T) {
	if !ExcludedDirs[".git"] {
		t.Error("Expected .git to be excluded")
	}
	if !ExcludedDirs[".hub"] {
		t.Error("Expected .hub to be excluded")
	}
	if ExcludedDirs["my-skill"] {
		t.Error("Expected my-skill to not be excluded")
	}
}

func TestLoadAllSkillsWithExcluded(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	// Create a valid skill
	validDir := filepath.Join(tmpDir, "skills", "valid-skill")
	os.MkdirAll(validDir, 0755)
	os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(`---
name: valid-skill
---
# Valid
`), 0644)

	// Create an excluded dir
	gitDir := filepath.Join(tmpDir, "skills", ".git")
	os.MkdirAll(filepath.Join(gitDir, "inner"), 0755)
	os.WriteFile(filepath.Join(gitDir, "inner", "SKILL.md"), []byte(`---
name: should-not-load
---
# Hidden
`), 0644)

	skills, err := LoadAllSkills()
	if err != nil {
		t.Fatalf("LoadAllSkills failed: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("Expected 1 skill (excluded .git), got %d", len(skills))
	}
	if skills[0].Meta.Name != "valid-skill" {
		t.Errorf("Expected 'valid-skill', got '%s'", skills[0].Meta.Name)
	}
}

func TestBuildSkillsIndex_ByDirName(t *testing.T) {
	skills := []*SkillEntry{
		{Meta: &SkillMeta{Name: "my-skill"}, DirName: "category/my-skill"},
	}

	index := BuildSkillsIndex(skills)

	// Should be findable by both name and dir name
	if _, ok := index["my-skill"]; !ok {
		t.Error("Expected index by name")
	}
	if _, ok := index["category/my-skill"]; !ok {
		t.Error("Expected index by dir name")
	}
}

// mockObjectStore is a minimal in-memory ObjectStore for unit tests.
type mockObjectStore struct {
	objects map[string][]byte
}

func (m *mockObjectStore) EnsureBucket(_ context.Context) error { return nil }
func (m *mockObjectStore) Bucket() string                       { return "test-bucket" }
func (m *mockObjectStore) Ping(_ context.Context) error         { return nil }
func (m *mockObjectStore) GetObject(_ context.Context, key string) ([]byte, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return data, nil
}
func (m *mockObjectStore) PutObject(_ context.Context, key string, data []byte) error {
	m.objects[key] = data
	return nil
}
func (m *mockObjectStore) DeleteObject(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}
func (m *mockObjectStore) ObjectExists(_ context.Context, key string) (bool, error) {
	_, ok := m.objects[key]
	return ok, nil
}
func (m *mockObjectStore) ListObjects(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.objects {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

const testSkillMD = `---
name: test-skill
description: A skill used in tests
version: 1.0.0
---
# Test Skill
Instructions.
`

// TestMinIOSkillLoader_ExcludesUserPaths verifies that LoadAll on a tenant-level
// MinIOSkillLoader does not return skill entries stored under the users/ sub-namespace.
// Regression test for: Skills API returning each skill twice when user-scoped copies exist.
func TestMinIOSkillLoader_ExcludesUserPaths(t *testing.T) {
	const tenantID = "tenant-abc"
	const userID = "user-xyz"

	store := &mockObjectStore{objects: map[string][]byte{
		// Tenant-level skill (should appear in results).
		tenantID + "/research/arxiv/SKILL.md": []byte(testSkillMD),
		// User-scoped copy of the same skill (must NOT appear in results).
		tenantID + "/users/" + userID + "/research/arxiv/SKILL.md": []byte(testSkillMD),
		// Another tenant-level skill to confirm non-user entries still load.
		tenantID + "/devops/docker/SKILL.md": []byte(`---
name: docker
description: Docker skill
---
# Docker
`),
	}}

	loader := NewMinIOSkillLoader(store, tenantID)
	entries, err := loader.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Meta.Name
		}
		t.Errorf("Expected 2 entries (tenant-level only), got %d: %v", len(entries), names)
	}

	for _, e := range entries {
		if strings.Contains(e.Meta.Path, "/users/") {
			t.Errorf("Entry %q has a user-scoped path %q; should have been filtered out", e.Meta.Name, e.Meta.Path)
		}
	}
}
