package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/objstore"
	skillspkg "github.com/Colin4k1024/hermesx/internal/skills"
)

const (
	maxSkillInstallFiles     = 100
	maxSkillInstallFileBytes = 1 << 20
	maxSkillInstallTotal     = 5 << 20
)

type skillInstallFile struct {
	RelPath string
	Data    []byte
}

type githubContentItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

func init() {
	Register(&ToolEntry{
		Name:    "skill_install",
		Toolset: "skills",
		Schema: map[string]any{
			"name":        "skill_install",
			"description": "Download and install a skill into SaaS object storage. Supports GitHub repositories such as https://github.com/anthropics/skills with a skill name, or direct HTTP(S) SKILL.md URLs. Use this when the user asks to install, add, download, or import a skill.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_url": map[string]any{
						"type":        "string",
						"description": "GitHub repository/tree URL or direct SKILL.md URL.",
					},
					"skill": map[string]any{
						"type":        "string",
						"description": "Skill directory name, for example frontend-design.",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "Install scope. tenant makes the skill available to the tenant; user installs it only for the current user. Defaults to tenant.",
						"enum":        []string{"tenant", "user"},
					},
					"command": map[string]any{
						"type":        "string",
						"description": "Optional command text to parse, such as npx skills add https://github.com/anthropics/skills --skill frontend-design.",
					},
				},
			},
		},
		Handler:      handleSkillInstall,
		Emoji:        "\U0001f9e9",
		MaxRedirects: 3,
	})
}

func handleSkillInstall(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	if tctx == nil || tctx.ObjectStore == nil {
		return `{"error":"object store is not configured for skill installation"}`
	}
	if strings.TrimSpace(tctx.TenantID) == "" {
		return `{"error":"tenant id is required for skill installation"}`
	}

	sourceURL, _ := args["source_url"].(string)
	skillName, _ := args["skill"].(string)
	if command, _ := args["command"].(string); strings.TrimSpace(command) != "" {
		cmdURL, cmdSkill := parseSkillAddCommand(command)
		if sourceURL == "" {
			sourceURL = cmdURL
		}
		if skillName == "" {
			skillName = cmdSkill
		}
	}
	sourceURL = strings.TrimSpace(sourceURL)
	skillName = sanitizeSkillName(skillName)
	if sourceURL == "" {
		return `{"error":"source_url is required"}`
	}

	scope, _ := args["scope"].(string)
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		scope = "tenant"
	}
	if scope != "tenant" && scope != "user" {
		return `{"error":"scope must be tenant or user"}`
	}
	if scope == "user" && strings.TrimSpace(tctx.UserID) == "" {
		return `{"error":"user scope requires user id"}`
	}

	client := tctx.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	files, resolvedName, err := downloadSkillFiles(ctx, client, sourceURL, skillName, tctx.AllowPrivateIPs)
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}
	resolvedName = sanitizeSkillName(resolvedName)
	if resolvedName == "" {
		return `{"error":"could not resolve skill name"}`
	}
	if !hasSkillMD(files) {
		return `{"error":"downloaded skill does not contain SKILL.md"}`
	}

	// Security scan: check SKILL.md content for injection before persisting.
	if tctx.Interceptor != nil {
		skillContent := extractSkillMDContent(files)
		result, err := tctx.Interceptor.ScanSkillContent(ctx, tctx.TenantID, resolvedName, skillContent)
		if err != nil {
			return toJSON(map[string]any{"error": fmt.Sprintf("security scan failed: %v", err)})
		}
		if !result.Allowed {
			return toJSON(map[string]any{"error": fmt.Sprintf("skill blocked by security policy: %s", result.Reason)})
		}
	}

	uploaded, keys, err := uploadSkillFiles(ctx, tctx.ObjectStore, tctx.TenantID, tctx.UserID, scope, resolvedName, files)
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}
	if scope == "tenant" {
		_ = skillspkg.MarkSkillUserModified(ctx, tctx.ObjectStore, tctx.TenantID, resolvedName)
	}

	return toJSON(map[string]any{
		"success":        true,
		"skill":          resolvedName,
		"scope":          scope,
		"uploaded_files": uploaded,
		"object_keys":    keys,
		"message":        fmt.Sprintf("Skill %q installed to %s object storage", resolvedName, scope),
	})
}

func parseSkillAddCommand(command string) (string, string) {
	parts := strings.Fields(command)
	var sourceURL, skillName string
	for i, part := range parts {
		if sourceURL == "" && (strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://")) {
			sourceURL = part
		}
		if part == "--skill" && i+1 < len(parts) {
			skillName = parts[i+1]
		}
		if strings.HasPrefix(part, "--skill=") {
			skillName = strings.TrimPrefix(part, "--skill=")
		}
	}
	return sourceURL, skillName
}

func downloadSkillFiles(ctx context.Context, client *http.Client, sourceURL, skillName string, allowPrivateIPs ...bool) ([]skillInstallFile, string, error) {
	u, err := url.Parse(sourceURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, "", fmt.Errorf("invalid source_url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", fmt.Errorf("source_url must be http or https")
	}
	if strings.EqualFold(u.Host, "github.com") {
		return downloadGitHubSkillFiles(ctx, client, u, skillName, allowPrivateIPs...)
	}
	if skillName == "" {
		skillName = deriveSkillNameFromURLPath(u.Path)
	}
	data, err := downloadFile(ctx, client, sourceURL, maxSkillInstallFileBytes, allowPrivateIPs...)
	if err != nil {
		return nil, "", err
	}
	return []skillInstallFile{{RelPath: "SKILL.md", Data: data}}, skillName, nil
}

func downloadGitHubSkillFiles(ctx context.Context, client *http.Client, u *url.URL, skillName string, allowPrivateIPs ...bool) ([]skillInstallFile, string, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("github source must include owner and repo")
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	ref := "main"
	basePath := ""
	if len(parts) >= 5 && parts[2] == "tree" {
		ref = parts[3]
		basePath = path.Clean(strings.Join(parts[4:], "/"))
		if basePath == "." {
			basePath = ""
		}
	}

	candidates := githubSkillCandidatePaths(basePath, skillName)
	var lastErr error
	for _, candidate := range candidates {
		files, err := githubCollectDir(ctx, client, owner, repo, ref, candidate, candidate, allowPrivateIPs...)
		if err == nil {
			resolvedName := skillName
			if resolvedName == "" {
				resolvedName = path.Base(candidate)
			}
			return files, resolvedName, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("could not locate skill in github repository")
}

func githubSkillCandidatePaths(basePath, skillName string) []string {
	skillName = sanitizeSkillName(skillName)
	if basePath == "" {
		if skillName == "" {
			return nil
		}
		return []string{path.Join("skills", skillName), skillName}
	}
	if skillName == "" || path.Base(basePath) == skillName {
		return []string{basePath}
	}
	return []string{path.Join(basePath, skillName), path.Join(basePath, "skills", skillName)}
}

func githubCollectDir(ctx context.Context, client *http.Client, owner, repo, ref, dirPath, rootPath string, allowPrivateIPs ...bool) ([]skillInstallFile, error) {
	items, err := githubListContents(ctx, client, owner, repo, ref, dirPath)
	if err != nil {
		return nil, err
	}
	var files []skillInstallFile
	var total int
	for _, item := range items {
		switch item.Type {
		case "dir":
			nested, err := githubCollectDir(ctx, client, owner, repo, ref, item.Path, rootPath)
			if err != nil {
				return nil, err
			}
			files = append(files, nested...)
		case "file":
			if item.DownloadURL == "" {
				continue
			}
			if item.Size > maxSkillInstallFileBytes {
				return nil, fmt.Errorf("skill file too large: %s", item.Path)
			}
			data, err := downloadFile(ctx, client, item.DownloadURL, maxSkillInstallFileBytes, allowPrivateIPs...)
			if err != nil {
				return nil, fmt.Errorf("download %s: %w", item.Path, err)
			}
			total += len(data)
			if total > maxSkillInstallTotal {
				return nil, fmt.Errorf("skill exceeds total size limit")
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(item.Path, rootPath), "/")
			files = append(files, skillInstallFile{RelPath: rel, Data: data})
			if len(files) > maxSkillInstallFiles {
				return nil, fmt.Errorf("skill has too many files")
			}
		}
	}
	return files, nil
}

func githubListContents(ctx context.Context, client *http.Client, owner, repo, ref, dirPath string) ([]githubContentItem, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, strings.TrimPrefix(dirPath, "/"), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hermesx-skill-install")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github contents request failed for %s: HTTP %d", dirPath, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSkillInstallFileBytes))
	if err != nil {
		return nil, err
	}
	var items []githubContentItem
	if err := json.Unmarshal(body, &items); err != nil {
		var item githubContentItem
		if err2 := json.Unmarshal(body, &item); err2 != nil {
			return nil, err
		}
		items = []githubContentItem{item}
	}
	return items, nil
}

func downloadFile(ctx context.Context, client *http.Client, rawURL string, maxBytes int64, allowPrivateIPs ...bool) ([]byte, error) {
	if len(allowPrivateIPs) == 0 || !allowPrivateIPs[0] {
		if safe, reason := IsSafeURL(rawURL); !safe {
			return nil, fmt.Errorf("blocked unsafe URL: %s", reason)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hermesx-skill-install")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("downloaded file exceeds size limit")
	}
	return data, nil
}

func hasSkillMD(files []skillInstallFile) bool {
	for _, file := range files {
		if strings.EqualFold(path.Base(file.RelPath), "SKILL.md") {
			return true
		}
	}
	return false
}

// extractSkillMDContent returns the body of SKILL.md from the file set.
func extractSkillMDContent(files []skillInstallFile) string {
	for _, file := range files {
		if strings.EqualFold(path.Base(file.RelPath), "SKILL.md") {
			return string(file.Data)
		}
	}
	return ""
}

func uploadSkillFiles(ctx context.Context, store objstore.ObjectStore, tenantID, userID, scope, skillName string, files []skillInstallFile) (int, []string, error) {
	prefix := tenantID + "/"
	if scope == "user" {
		prefix += "users/" + userID + "/"
	}
	prefix += skillName + "/"

	keys := make([]string, 0, len(files))
	for _, file := range files {
		rel, ok := cleanSkillRelPath(file.RelPath)
		if !ok {
			return 0, nil, fmt.Errorf("unsafe skill file path: %s", file.RelPath)
		}
		key := prefix + rel
		if err := store.PutObject(ctx, key, file.Data); err != nil {
			return len(keys), keys, fmt.Errorf("upload %s: %w", key, err)
		}
		keys = append(keys, key)
	}
	return len(keys), keys, nil
}

func cleanSkillRelPath(rel string) (string, bool) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean == ".." {
		return "", false
	}
	return clean, true
}

func sanitizeSkillName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".md")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-_")
	return strings.ToLower(name)
}

func deriveSkillNameFromURLPath(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if strings.EqualFold(part, "SKILL.md") && i > 0 {
			return sanitizeSkillName(parts[i-1])
		}
		if part != "" {
			return sanitizeSkillName(part)
		}
	}
	return ""
}
