package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsList(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	// Create a skill
	skillDir := filepath.Join(tmpDir, "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test
---
# Test
`), 0644)

	result := handleSkillsList(context.Background(), map[string]any{}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)

	skills, _ := m["skills"].([]any)
	if len(skills) == 0 {
		t.Error("Expected at least 1 skill")
	}
}

func TestSkillsListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)

	result := handleSkillsList(context.Background(), map[string]any{}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)
	// Should not error
	if m["error"] != nil {
		t.Errorf("Unexpected error: %v", m["error"])
	}
}

func TestSkillView(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillDir := filepath.Join(tmpDir, "skills", "viewable")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: viewable
description: Viewable skill
---
# Viewable Skill
Instructions here.
`), 0644)

	result := handleSkillView(context.Background(), map[string]any{"name": "viewable"}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)
	if m["error"] != nil {
		t.Errorf("Unexpected error: %v", m["error"])
	}
}

func TestSkillViewNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)

	result := handleSkillView(context.Background(), map[string]any{"name": "nonexistent"}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)
	if m["error"] == nil {
		t.Error("Expected error for nonexistent skill")
	}
}

func TestSkillManageCreate(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)

	result := handleSkillManage(context.Background(), map[string]any{
		"action":      "create",
		"name":        "new-skill",
		"description": "A brand new skill",
		"content":     "# New Skill\nDo stuff.",
	}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)
	if m["success"] != true {
		t.Errorf("Expected create success, got: %s", result)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "skills", "new-skill", "SKILL.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Expected SKILL.md to exist after create")
	}
}

func TestSkillManageDelete(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	skillDir := filepath.Join(tmpDir, "skills", "to-delete")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Delete me"), 0644)

	result := handleSkillManage(context.Background(), map[string]any{
		"action": "delete",
		"name":   "to-delete",
	}, nil)
	var m map[string]any
	json.Unmarshal([]byte(result), &m)
	if m["success"] != true {
		t.Errorf("Expected delete success, got: %s", result)
	}
}

func TestSkillManageSaaSUsesObjectStore(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		ObjectStore: store,
	}

	create := handleSkillManage(context.Background(), map[string]any{
		"action":  "create",
		"name":    "tenant-skill",
		"content": "---\ndescription: Tenant skill\n---\n# Tenant Skill\n",
	}, tctx)
	if !jsonContains(create, `"success":true`) {
		t.Fatalf("expected create success, got %s", create)
	}
	if _, ok := store.objects["tenant-1/tenant-skill/SKILL.md"]; !ok {
		t.Fatalf("expected tenant skill object, objects=%#v", store.objects)
	}

	list := handleSkillsList(context.Background(), map[string]any{}, tctx)
	if !jsonContains(list, `"tenant_id":"tenant-1"`) || !jsonContains(list, `"name":"tenant-skill"`) {
		t.Fatalf("expected tenant skill in list, got %s", list)
	}

	view := handleSkillView(context.Background(), map[string]any{"name": "tenant-skill"}, tctx)
	if !jsonContains(view, `"key":"tenant-1/tenant-skill/SKILL.md"`) {
		t.Fatalf("expected tenant skill object key, got %s", view)
	}
}

func TestSkillsListSaaS_TenantAndUserSkills(t *testing.T) {
	store := newFakeSkillObjectStore()
	store.objects["tenant-1/shared-skill/SKILL.md"] = []byte("# Shared Skill\nTenant level.")
	store.objects["tenant-1/tenant-only/SKILL.md"] = []byte("# Tenant Only\nOnly in tenant.")
	store.objects["tenant-1/users/user-1/user-skill/SKILL.md"] = []byte("# User Skill\nUser level.")
	store.objects["tenant-1/users/user-1/shared-skill/SKILL.md"] = []byte("# Shared Skill\nUser override.")

	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: store,
	}

	result := handleSkillsList(context.Background(), map[string]any{}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	skills, ok := m["skills"].([]any)
	if !ok {
		t.Fatalf("expected skills array, got %s", result)
	}

	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d: %s", len(skills), result)
	}

	skillMap := make(map[string]map[string]any)
	for _, s := range skills {
		skill := s.(map[string]any)
		skillMap[skill["name"].(string)] = skill
	}

	if _, ok := skillMap["tenant-only"]; !ok {
		t.Error("expected tenant-only skill in list")
	}
	if _, ok := skillMap["user-skill"]; !ok {
		t.Error("expected user-skill in list")
	}
	sharedSkill, ok := skillMap["shared-skill"]
	if !ok {
		t.Error("expected shared-skill in list")
	} else if sharedSkill["is_user_skill"] != true {
		t.Error("expected shared-skill to be user skill (user override)")
	}
}

func TestSkillsListSaaS_OnlyUserSkills(t *testing.T) {
	store := newFakeSkillObjectStore()
	store.objects["tenant-1/users/user-1/my-skill/SKILL.md"] = []byte("# My Skill\nPersonal skill.")

	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: store,
	}

	result := handleSkillsList(context.Background(), map[string]any{}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	skills, ok := m["skills"].([]any)
	if !ok || len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %s", result)
	}

	skill := skills[0].(map[string]any)
	if skill["name"] != "my-skill" {
		t.Errorf("expected my-skill, got %s", skill["name"])
	}
	if skill["is_user_skill"] != true {
		t.Error("expected is_user_skill=true")
	}
}

func TestSkillsListSaaS_NoUserID(t *testing.T) {
	store := newFakeSkillObjectStore()
	store.objects["tenant-1/tenant-skill/SKILL.md"] = []byte("# Tenant Skill")
	store.objects["tenant-1/users/user-1/user-skill/SKILL.md"] = []byte("# User Skill")

	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "",
		ObjectStore: store,
	}

	result := handleSkillsList(context.Background(), map[string]any{}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	skills, ok := m["skills"].([]any)
	if !ok || len(skills) != 1 {
		t.Fatalf("expected 1 skill (tenant only), got %s", result)
	}

	skill := skills[0].(map[string]any)
	if skill["name"] != "tenant-skill" {
		t.Errorf("expected tenant-skill, got %s", skill["name"])
	}
}

func TestSkillViewSaaS_FallbackToUserSkill(t *testing.T) {
	store := newFakeSkillObjectStore()
	store.objects["tenant-1/users/user-1/personal-skill/SKILL.md"] = []byte("# Personal Skill\nUser content.")

	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: store,
	}

	result := handleSkillView(context.Background(), map[string]any{"name": "personal-skill"}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if m["error"] != nil {
		t.Fatalf("unexpected error: %v", m["error"])
	}
	if m["content"] != "# Personal Skill\nUser content." {
		t.Errorf("unexpected content: %v", m["content"])
	}
	if m["key"] != "tenant-1/users/user-1/personal-skill/SKILL.md" {
		t.Errorf("unexpected key: %v", m["key"])
	}
}

func TestSkillViewSaaS_TenantSkill优先(t *testing.T) {
	store := newFakeSkillObjectStore()
	store.objects["tenant-1/shared-skill/SKILL.md"] = []byte("# Shared Skill\nTenant version.")
	store.objects["tenant-1/users/user-1/shared-skill/SKILL.md"] = []byte("# Shared Skill\nUser version.")

	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: store,
	}

	result := handleSkillView(context.Background(), map[string]any{"name": "shared-skill"}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if m["error"] != nil {
		t.Fatalf("unexpected error: %v", m["error"])
	}
	if m["content"] != "# Shared Skill\nTenant version." {
		t.Errorf("expected tenant version, got: %v", m["content"])
	}
	if m["key"] != "tenant-1/shared-skill/SKILL.md" {
		t.Errorf("expected tenant key, got: %v", m["key"])
	}
}

func TestSkillViewSaaS_NotFound(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: store,
	}

	result := handleSkillView(context.Background(), map[string]any{"name": "nonexistent"}, tctx)

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if m["error"] == nil {
		t.Error("expected error for nonexistent skill")
	}
}
