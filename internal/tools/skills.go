package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/config"
	skillspkg "github.com/Colin4k1024/hermesx/internal/skills"
)

func init() {
	Register(&ToolEntry{
		Name:    "skills_list",
		Toolset: "skills",
		Schema: map[string]any{
			"name":        "skills_list",
			"description": "List tenant skills available to the SaaS agent runtime. Shows skill name, description, and status.",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Handler: handleSkillsList,
		Emoji:   "\U0001f4da",
	})

	Register(&ToolEntry{
		Name:    "skill_view",
		Toolset: "skills",
		Schema: map[string]any{
			"name":        "skill_view",
			"description": "Read the contents of a specific skill's SKILL.md file.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the skill to view",
					},
				},
				"required": []string{"name"},
			},
		},
		Handler: handleSkillView,
		Emoji:   "\U0001f4d6",
	})

	Register(&ToolEntry{
		Name:    "skill_manage",
		Toolset: "skills",
		Schema: map[string]any{
			"name":        "skill_manage",
			"description": "Create, edit, or delete a tenant skill in SaaS object storage.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"create", "edit", "delete"},
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Skill name (used as directory name)",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "SKILL.md content (for create/edit)",
					},
				},
				"required": []string{"action", "name"},
			},
		},
		Handler: handleSkillManage,
		Emoji:   "\u2699\ufe0f",
	})
}

func getSkillsDir() string {
	return filepath.Join(config.HermesHome(), "skills")
}

func handleSkillsList(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	if tctx != nil && tctx.ObjectStore != nil {
		return handleSkillsListSaaS(ctx, tctx)
	}

	skillsDir := getSkillsDir()
	os.MkdirAll(skillsDir, 0755)

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Cannot read skills directory: %v", err)})
	}

	var skills []map[string]any
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillMD := filepath.Join(skillsDir, skillName, "SKILL.md")

		skill := map[string]any{
			"name":   skillName,
			"path":   filepath.Join(skillsDir, skillName),
			"has_md": fileExists(skillMD),
		}

		// Try to read first line as description
		if data, err := os.ReadFile(skillMD); err == nil {
			lines := strings.SplitN(string(data), "\n", 3)
			for _, line := range lines {
				trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
				if trimmed != "" && !strings.HasPrefix(line, "#") {
					skill["description"] = truncateOutput(trimmed, 200)
					break
				}
			}
			skill["size"] = len(data)
		}

		skills = append(skills, skill)
	}

	return toJSON(map[string]any{
		"skills":     skills,
		"count":      len(skills),
		"skills_dir": skillsDir,
	})
}

func handleSkillView(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	name, _ := args["name"].(string)
	if name == "" {
		return `{"error":"name is required"}`
	}
	if tctx != nil && tctx.ObjectStore != nil {
		return handleSkillViewSaaS(ctx, tctx, name)
	}

	skillMD := filepath.Join(getSkillsDir(), name, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Cannot read skill '%s': %v", name, err),
			"hint":  "Use skills_list to see available skills",
		})
	}

	return toJSON(map[string]any{
		"name":    name,
		"content": string(data),
		"path":    skillMD,
	})
}

func handleSkillManage(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	action, _ := args["action"].(string)
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)

	if name == "" {
		return `{"error":"name is required"}`
	}

	// Sanitize skill name
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "..", "")

	if tctx != nil && tctx.ObjectStore != nil {
		return handleSkillManageSaaS(ctx, action, name, content, tctx)
	}

	skillDir := filepath.Join(getSkillsDir(), name)
	skillMD := filepath.Join(skillDir, "SKILL.md")

	switch action {
	case "create":
		if content == "" {
			return `{"error":"content is required for create"}`
		}
		if fileExists(skillDir) {
			return toJSON(map[string]any{
				"error": fmt.Sprintf("Skill '%s' already exists. Use 'edit' to modify.", name),
			})
		}
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot create skill directory: %v", err)})
		}
		if err := os.WriteFile(skillMD, []byte(content), 0644); err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot write SKILL.md: %v", err)})
		}
		return toJSON(map[string]any{
			"success": true,
			"name":    name,
			"path":    skillMD,
			"message": fmt.Sprintf("Skill '%s' created successfully", name),
		})

	case "edit":
		if content == "" {
			return `{"error":"content is required for edit"}`
		}
		if !fileExists(skillDir) {
			return toJSON(map[string]any{
				"error": fmt.Sprintf("Skill '%s' does not exist. Use 'create' first.", name),
			})
		}
		if err := os.WriteFile(skillMD, []byte(content), 0644); err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot write SKILL.md: %v", err)})
		}
		return toJSON(map[string]any{
			"success": true,
			"name":    name,
			"path":    skillMD,
			"message": fmt.Sprintf("Skill '%s' updated successfully", name),
		})

	case "delete":
		if !fileExists(skillDir) {
			return toJSON(map[string]any{
				"error": fmt.Sprintf("Skill '%s' does not exist", name),
			})
		}
		if err := os.RemoveAll(skillDir); err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot delete skill: %v", err)})
		}
		return toJSON(map[string]any{
			"success": true,
			"name":    name,
			"message": fmt.Sprintf("Skill '%s' deleted successfully", name),
		})

	default:
		return `{"error":"Invalid action. Use: create, edit, delete"}`
	}
}

func handleSkillsListSaaS(ctx context.Context, tctx *ToolContext) string {
	if strings.TrimSpace(tctx.TenantID) == "" {
		return `{"error":"tenant id is required for skills_list"}`
	}
	prefix := tctx.TenantID + "/"
	keys, err := tctx.ObjectStore.ListObjects(ctx, prefix)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to list tenant skills: %v", err)})
	}

	var skills []map[string]any
	for _, key := range keys {
		if !strings.HasSuffix(key, "/SKILL.md") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, prefix), "/SKILL.md")
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		skill := map[string]any{
			"name":   name,
			"key":    key,
			"has_md": true,
		}
		if data, err := tctx.ObjectStore.GetObject(ctx, key); err == nil {
			if desc := firstSkillDescription(string(data)); desc != "" {
				skill["description"] = desc
			}
			skill["size"] = len(data)
		}
		skills = append(skills, skill)
	}

	return toJSON(map[string]any{
		"tenant_id": tctx.TenantID,
		"skills":    skills,
		"count":     len(skills),
	})
}

func handleSkillViewSaaS(ctx context.Context, tctx *ToolContext, name string) string {
	if strings.TrimSpace(tctx.TenantID) == "" {
		return `{"error":"tenant id is required for skill_view"}`
	}
	name = sanitizeSkillName(name)
	key := skillObjectKey(tctx.TenantID, name)
	data, err := tctx.ObjectStore.GetObject(ctx, key)
	if err != nil {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Cannot read tenant skill %q: %v", name, err),
			"hint":  "Use skills_list to see available tenant skills",
		})
	}
	return toJSON(map[string]any{
		"name":    name,
		"content": string(data),
		"key":     key,
	})
}

func handleSkillManageSaaS(ctx context.Context, action, name, content string, tctx *ToolContext) string {
	if strings.TrimSpace(tctx.TenantID) == "" {
		return `{"error":"tenant id is required for skill_manage"}`
	}
	name = sanitizeSkillName(name)
	if name == "" {
		return `{"error":"valid skill name is required"}`
	}
	key := skillObjectKey(tctx.TenantID, name)

	switch action {
	case "create":
		if content == "" {
			return `{"error":"content is required for create"}`
		}
		exists, err := tctx.ObjectStore.ObjectExists(ctx, key)
		if err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot check skill object: %v", err)})
		}
		if exists {
			return toJSON(map[string]any{
				"error": fmt.Sprintf("Skill %q already exists. Use edit to modify.", name),
			})
		}
		return putTenantSkill(ctx, tctx, key, name, content, "created")
	case "edit":
		if content == "" {
			return `{"error":"content is required for edit"}`
		}
		exists, err := tctx.ObjectStore.ObjectExists(ctx, key)
		if err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot check skill object: %v", err)})
		}
		if !exists {
			return toJSON(map[string]any{
				"error": fmt.Sprintf("Skill %q does not exist. Use create first.", name),
			})
		}
		return putTenantSkill(ctx, tctx, key, name, content, "updated")
	case "delete":
		exists, err := tctx.ObjectStore.ObjectExists(ctx, key)
		if err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot check skill object: %v", err)})
		}
		if !exists {
			return toJSON(map[string]any{"error": fmt.Sprintf("Skill %q does not exist", name)})
		}
		if err := tctx.ObjectStore.DeleteObject(ctx, key); err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("Cannot delete skill object: %v", err)})
		}
		_ = skillspkg.MarkSkillUserModified(ctx, tctx.ObjectStore, tctx.TenantID, name)
		return toJSON(map[string]any{
			"success": true,
			"name":    name,
			"key":     key,
			"message": fmt.Sprintf("Skill %q deleted successfully", name),
		})
	default:
		return `{"error":"Invalid action. Use: create, edit, delete"}`
	}
}

func putTenantSkill(ctx context.Context, tctx *ToolContext, key, name, content, action string) string {
	if err := tctx.ObjectStore.PutObject(ctx, key, []byte(content)); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Cannot write tenant skill: %v", err)})
	}
	_ = skillspkg.MarkSkillUserModified(ctx, tctx.ObjectStore, tctx.TenantID, name)
	return toJSON(map[string]any{
		"success": true,
		"name":    name,
		"key":     key,
		"message": fmt.Sprintf("Skill %q %s successfully", name, action),
	})
}

func skillObjectKey(tenantID, name string) string {
	return fmt.Sprintf("%s/%s/SKILL.md", tenantID, sanitizeSkillName(name))
}

func firstSkillDescription(content string) string {
	lines := strings.SplitN(content, "\n", 12)
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if trimmed != "" && !strings.HasPrefix(line, "#") {
			return truncateOutput(trimmed, 200)
		}
	}
	return ""
}
