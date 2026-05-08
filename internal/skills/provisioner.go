package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// TenantManifest tracks which skills a tenant has and their modification state.
type TenantManifest struct {
	Version  int                        `json:"version"`
	Skills   map[string]TenantSkillMeta `json:"skills"`
	SyncedAt time.Time                  `json:"synced_at"`
}

// TenantSkillMeta holds per-skill metadata stored in the manifest.
type TenantSkillMeta struct {
	Source       string    `json:"source"`
	UserModified bool      `json:"user_modified"`
	InstalledAt  time.Time `json:"installed_at"`
}

const manifestKey = "/.manifest.json"

func loadManifest(ctx context.Context, mc objstore.ObjectStore, tenantID string) (*TenantManifest, error) {
	data, err := mc.GetObject(ctx, tenantID+manifestKey)
	if err != nil {
		return &TenantManifest{Version: 1, Skills: make(map[string]TenantSkillMeta)}, nil
	}
	var m TenantManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &TenantManifest{Version: 1, Skills: make(map[string]TenantSkillMeta)}, nil
	}
	if m.Skills == nil {
		m.Skills = make(map[string]TenantSkillMeta)
	}
	return &m, nil
}

func saveManifest(ctx context.Context, mc objstore.ObjectStore, tenantID string, m *TenantManifest) error {
	m.SyncedAt = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return mc.PutObject(ctx, tenantID+manifestKey, data)
}

// LoadTenantManifestPublic returns the tenant skill manifest for read-only use.
func LoadTenantManifestPublic(ctx context.Context, mc objstore.ObjectStore, tenantID string) (*TenantManifest, error) {
	return loadManifest(ctx, mc, tenantID)
}

// MarkSkillUserModified flags a skill as manually modified by the tenant user.
func MarkSkillUserModified(ctx context.Context, mc objstore.ObjectStore, tenantID, skillName string) error {
	m, err := loadManifest(ctx, mc, tenantID)
	if err != nil {
		return err
	}
	meta := m.Skills[skillName]
	meta.UserModified = true
	if meta.InstalledAt.IsZero() {
		meta.InstalledAt = time.Now()
	}
	m.Skills[skillName] = meta
	return saveManifest(ctx, mc, tenantID, m)
}

// defaultSoulContent mirrors cli.DefaultSoulMD to avoid import cycle (skills → cli → agent → skills).
const defaultSoulContent = `# Hermes Agent

You are Hermes, an AI assistant built by Nous Research.

## Core Identity
- You are helpful, accurate, and proactive.
- You use available tools to accomplish tasks effectively.
- You prioritize user safety and warn before destructive operations.
- You are transparent about your capabilities and limitations.

## Personality
- Friendly but professional tone.
- Concise responses unless detail is requested.
- Offer actionable suggestions when appropriate.
- Admit uncertainty rather than guessing.

## Principles
1. Use tools when they can provide better answers than your training data alone.
2. Ask for clarification when instructions are ambiguous.
3. Break complex tasks into manageable steps.
4. Preserve user data and avoid unintended side effects.
5. Respect privacy — never log or transmit sensitive information unnecessarily.
`

const maxSoulBytes = 64 * 1024

type Provisioner struct {
	minio      objstore.ObjectStore
	bundledDir string
}

func validateTenantID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid tenant ID: %q", id)
	}
	return nil
}

func NewProvisioner(minio objstore.ObjectStore, bundledDir string) *Provisioner {
	if envDir := os.Getenv("HERMES_SKILLS_DIR"); envDir != "" {
		bundledDir = envDir
	}
	if bundledDir != "" && !filepath.IsAbs(bundledDir) {
		if abs, err := filepath.Abs(bundledDir); err == nil {
			bundledDir = abs
		}
	}
	return &Provisioner{minio: minio, bundledDir: bundledDir}
}

func (p *Provisioner) Provision(ctx context.Context, tenantID string) error {
	if err := validateTenantID(tenantID); err != nil {
		return err
	}
	var errs []string
	if err := p.ProvisionSoul(ctx, tenantID); err != nil {
		slog.Error("provision soul failed", "tenant", tenantID, "error", err)
		errs = append(errs, err.Error())
	}
	if err := p.ProvisionSkills(ctx, tenantID); err != nil {
		slog.Error("provision skills failed", "tenant", tenantID, "error", err)
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("provision partial failure: %s", strings.Join(errs, "; "))
	}
	slog.Info("tenant provisioning complete", "tenant", tenantID)
	return nil
}

func (p *Provisioner) ProvisionSoul(ctx context.Context, tenantID string) error {
	key := tenantID + "/SOUL.md"
	exists, err := p.minio.ObjectExists(ctx, key)
	if err != nil {
		return fmt.Errorf("check soul exists: %w", err)
	}
	if exists {
		slog.Debug("soul already exists, skipping", "tenant", tenantID)
		return nil
	}
	if err := p.minio.PutObject(ctx, key, []byte(defaultSoulContent)); err != nil {
		return fmt.Errorf("upload soul: %w", err)
	}
	slog.Info("provisioned tenant soul", "tenant", tenantID, "key", key)
	return nil
}

func (p *Provisioner) ProvisionSkills(ctx context.Context, tenantID string) error {
	if p.bundledDir == "" {
		return nil
	}
	if _, err := os.Stat(p.bundledDir); os.IsNotExist(err) {
		slog.Warn("bundled skills directory not found", "dir", p.bundledDir)
		return nil
	}

	m, err := loadManifest(ctx, p.minio, tenantID)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	var uploaded, skipped int
	now := time.Now()
	err = filepath.Walk(p.bundledDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), "SKILL.md") {
			return nil
		}

		rel, err := filepath.Rel(p.bundledDir, path)
		if err != nil {
			return nil
		}
		key := tenantID + "/" + filepath.ToSlash(rel)

		exists, err := p.minio.ObjectExists(ctx, key)
		if err != nil {
			slog.Warn("check skill exists failed", "key", key, "error", err)
			return nil
		}
		if exists {
			skipped++
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("read bundled skill failed", "path", path, "error", err)
			return nil
		}
		if err := p.minio.PutObject(ctx, key, data); err != nil {
			slog.Warn("upload skill failed", "key", key, "error", err)
			return nil
		}

		// Record skill in manifest with bundled source.
		skillDir := filepath.ToSlash(filepath.Dir(rel))
		if _, ok := m.Skills[skillDir]; !ok {
			m.Skills[skillDir] = TenantSkillMeta{
				Source:      "bundled",
				InstalledAt: now,
			}
		}
		uploaded++
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk bundled skills: %w", err)
	}

	if uploaded > 0 {
		if merr := saveManifest(ctx, p.minio, tenantID, m); merr != nil {
			slog.Warn("save manifest failed after skill provision", "tenant", tenantID, "error", merr)
		}
	}

	slog.Info("tenant skill sync complete", "tenant", tenantID, "uploaded", uploaded, "skipped", skipped)
	return nil
}

// ProvisionUserSkills copies all tenant-level skills into the user-scoped OSS namespace,
// enabling per-user auto-loading. The operation is idempotent: a marker object is written
// after the first successful provision and checked on subsequent calls.
//
// OSS layout:
//
//	{tenantID}/{category}/{skillName}/SKILL.md  (tenant source)
//	{tenantID}/users/{userID}/{category}/{skillName}/SKILL.md  (user copy)
//	{tenantID}/users/{userID}/.initialized  (idempotency marker)
func (p *Provisioner) ProvisionUserSkills(ctx context.Context, tenantID, userID string) error {
	if err := validateTenantID(tenantID); err != nil {
		return err
	}
	if userID == "" || userID == "." || userID == ".." || strings.ContainsAny(userID, "/\\") {
		return fmt.Errorf("invalid user ID: %q", userID)
	}

	markerKey := tenantID + "/users/" + userID + "/.initialized"
	if ok, _ := p.minio.ObjectExists(ctx, markerKey); ok {
		slog.Debug("user skills already provisioned, skipping", "tenant", tenantID, "user", userID)
		return nil
	}

	// List all objects under the tenant prefix.
	keys, err := p.minio.ListObjects(ctx, tenantID+"/")
	if err != nil {
		return fmt.Errorf("list tenant skills: %w", err)
	}

	userPrefix := tenantID + "/users/"
	var copied int
	for _, key := range keys {
		// Skip the manifest, SOUL.md, and anything already under /users/.
		if strings.HasPrefix(key, userPrefix) {
			continue
		}
		if !strings.HasSuffix(key, "/SKILL.md") {
			continue
		}

		// Build destination key: replace leading "{tenantID}/" with "{tenantID}/users/{userID}/".
		relPath := strings.TrimPrefix(key, tenantID+"/")
		destKey := userPrefix + userID + "/" + relPath

		// Skip if the user already has this skill (re-entrant copy after partial failure).
		if exists, _ := p.minio.ObjectExists(ctx, destKey); exists {
			continue
		}

		data, err := p.minio.GetObject(ctx, key)
		if err != nil {
			slog.Warn("user_skill_copy_get_failed", "src", key, "error", err)
			continue
		}
		if err := p.minio.PutObject(ctx, destKey, data); err != nil {
			slog.Warn("user_skill_copy_put_failed", "dst", destKey, "error", err)
			continue
		}
		copied++
	}

	// Write idempotency marker so we do not repeat on the next request.
	if err := p.minio.PutObject(ctx, markerKey, []byte{}); err != nil {
		slog.Warn("user_skill_marker_write_failed", "tenant", tenantID, "user", userID, "error", err)
		// Non-fatal: provisioning succeeded; marker failure means we will re-copy next time.
	}

	slog.Info("user skill provisioning complete", "tenant", tenantID, "user", userID, "copied", copied)
	return nil
}

// SyncAllTenants provisions all tenants with paginated listing and bounded concurrency.
func (p *Provisioner) SyncAllTenants(ctx context.Context, tenantStore store.TenantStore) error {
	const pageSize = 100
	const maxConcurrency = 10

	sem := make(chan struct{}, maxConcurrency)
	var total, failed int
	offset := 0

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		tenants, _, err := tenantStore.List(ctx, store.ListOptions{Limit: pageSize, Offset: offset})
		if err != nil {
			return fmt.Errorf("list tenants (offset=%d): %w", offset, err)
		}
		if len(tenants) == 0 {
			break
		}

		var wg sync.WaitGroup
		for _, t := range tenants {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(tenantID string) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := p.Provision(ctx, tenantID); err != nil {
					slog.Error("sync tenant failed", "tenant", tenantID, "error", err)
					failed++
				}
			}(t.ID)
		}
		wg.Wait()
		total += len(tenants)

		if len(tenants) < pageSize {
			break
		}
		offset += pageSize
	}

	slog.Info("tenant sync complete", "total", total, "failed", failed)
	return nil
}
