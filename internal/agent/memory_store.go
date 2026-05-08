package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// StoreMemoryProvider implements MemoryProvider backed by store.Store.
// Works with any store backend (MySQL, PostgreSQL, SQLite) — no direct
// pgxpool access required.
type StoreMemoryProvider struct {
	memories store.MemoryStore
	profiles store.UserProfileStore
	tenantID string
	userID   string
}

var _ MemoryProvider = (*StoreMemoryProvider)(nil)
var _ SystemPromptProvider = (*StoreMemoryProvider)(nil)
var _ ShutdownProvider = (*StoreMemoryProvider)(nil)

// NewStoreMemoryProvider creates a store-backed memory provider.
func NewStoreMemoryProvider(s store.Store, tenantID, userID string) *StoreMemoryProvider {
	return &StoreMemoryProvider{
		memories: s.Memories(),
		profiles: s.UserProfiles(),
		tenantID: tenantID,
		userID:   userID,
	}
}

func (p *StoreMemoryProvider) ReadMemory() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const maxEntries = 50
	const maxBytes = 8192

	entries, err := p.memories.List(ctx, p.tenantID, p.userID)
	if err != nil {
		return "", fmt.Errorf("store read memory: %w", err)
	}

	var parts []string
	totalBytes := 0
	for i, e := range entries {
		if i >= maxEntries {
			break
		}
		entry := fmt.Sprintf("## %s\n%s", e.Key, e.Content)
		if totalBytes+len(entry) > maxBytes {
			break
		}
		parts = append(parts, entry)
		totalBytes += len(entry)
	}

	slog.Debug("store_read_memory", "tenant", p.tenantID, "user", p.userID, "entries", len(parts))
	return strings.Join(parts, "\n\n"), nil
}

func (p *StoreMemoryProvider) SaveMemory(key, content string) error {
	if key == "" || content == "" {
		return fmt.Errorf("both key and content are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.memories.Upsert(ctx, p.tenantID, p.userID, key, content)
}

func (p *StoreMemoryProvider) DeleteMemory(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.memories.Delete(ctx, p.tenantID, p.userID, key)
}

func (p *StoreMemoryProvider) ReadUserProfile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	content, err := p.profiles.Get(ctx, p.tenantID, p.userID)
	if err != nil {
		return "", fmt.Errorf("store read user profile: %w", err)
	}
	return content, nil
}

func (p *StoreMemoryProvider) SaveUserProfile(content string) error {
	if content == "" {
		return fmt.Errorf("content is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.profiles.Upsert(ctx, p.tenantID, p.userID, content)
}

func (p *StoreMemoryProvider) SystemPromptBlock() string {
	var parts []string

	profile, err := p.ReadUserProfile()
	if err != nil {
		slog.Warn("store: failed to read user profile for system prompt", "error", err)
	} else if profile != "" {
		parts = append(parts, "## Known User Profile\nThe following is what you already know about this user. Use it to personalize responses.\n"+profile)
	}

	memory, err := p.ReadMemory()
	if err != nil {
		slog.Warn("store: failed to read memory for system prompt", "error", err)
	} else if memory != "" {
		parts = append(parts, "## Saved Memory\nThe following facts have been saved from previous conversations with this user.\n"+memory)
	}

	return strings.Join(parts, "\n\n")
}

func (p *StoreMemoryProvider) Shutdown() error { return nil }

// storeMemoryAdapter wraps StoreMemoryProvider to satisfy tools.MemoryProvider.
var _ SystemPromptProvider = (*storeMemoryAdapter)(nil)

type storeMemoryAdapter struct {
	inner *StoreMemoryProvider
}

func (a *storeMemoryAdapter) ReadMemory() (string, error)      { return a.inner.ReadMemory() }
func (a *storeMemoryAdapter) SaveMemory(k, c string) error     { return a.inner.SaveMemory(k, c) }
func (a *storeMemoryAdapter) DeleteMemory(k string) error      { return a.inner.DeleteMemory(k) }
func (a *storeMemoryAdapter) ReadUserProfile() (string, error) { return a.inner.ReadUserProfile() }
func (a *storeMemoryAdapter) SaveUserProfile(c string) error   { return a.inner.SaveUserProfile(c) }
func (a *storeMemoryAdapter) SystemPromptBlock() string        { return a.inner.SystemPromptBlock() }

// NewStoreMemoryProviderAsToolsProvider creates a tools.MemoryProvider backed by
// store.Store, scoped to a specific tenant and user. Works with MySQL, PostgreSQL,
// and SQLite backends.
func NewStoreMemoryProviderAsToolsProvider(s store.Store, tenantID, userID string) tools.MemoryProvider {
	return &storeMemoryAdapter{
		inner: NewStoreMemoryProvider(s, tenantID, userID),
	}
}
