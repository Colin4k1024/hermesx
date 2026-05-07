package skills

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
)

// InstallFromURL downloads and installs a skill from an HTTP(S) URL.
func InstallFromURL(rawURL string) (string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("URL must start with http:// or https://")
	}

	// Derive skill name from URL
	name := deriveNameFromURL(rawURL)
	if name == "" {
		return "", fmt.Errorf("cannot derive skill name from URL: %s", rawURL)
	}

	skillDir := filepath.Join(config.HermesHome(), "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("create skill dir: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("read response: %w", err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("save skill: %w", err)
	}

	// Security scan
	trust := TrustCommunity
	scanResult, scanErr := ScanSkillWithTrust(skillDir, trust)
	if scanErr == nil && scanResult != nil {
		decision := ShouldAllowInstall(trust, scanResult)
		if decision == InstallBlock {
			os.RemoveAll(skillDir)
			return "", fmt.Errorf("skill blocked by security scan: %s", FormatIssues(scanResult.Findings))
		}
	}

	writeLockEntry(name, rawURL)
	return name, nil
}

func deriveNameFromURL(rawURL string) string {
	// Extract filename from URL path
	parts := strings.Split(rawURL, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" || p == "SKILL.md" || p == "skill.md" {
			continue
		}
		// Remove file extensions
		name := strings.TrimSuffix(p, ".md")
		name = strings.TrimSuffix(name, ".txt")
		// Sanitize
		name = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, name)
		name = strings.Trim(name, "-")
		if name != "" {
			return strings.ToLower(name)
		}
	}
	return ""
}
