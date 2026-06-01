package mcpcatalog

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStore_UpsertListAndGet(t *testing.T) {
	store := NewMemoryStore()
	item := Item{
		ID:           "n8n",
		Name:         "n8n",
		SourceURL:    "https://github.com/modelcontextprotocol/servers",
		TrustTier:    TrustTrusted,
		ReviewStatus: ReviewApproved,
		Transport:    TransportStdio,
		Command:      "npx",
		Args:         []string{"-y", "@n8n/mcp"},
	}

	got, err := store.UpsertItem(context.Background(), item)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", got)
	}

	items, err := store.ListItems(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != "n8n" {
		t.Fatalf("items = %+v", items)
	}

	fetched, err := store.GetItem(context.Background(), "n8n")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Name != "n8n" {
		t.Fatalf("fetched = %+v", fetched)
	}
}

func TestMemoryStore_SetTenantPolicyRequiresExistingItem(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.SetTenantPolicy(context.Background(), TenantItemPolicy{
		TenantID: "tenant-a",
		ItemID:   "missing",
		Enabled:  true,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestValidateItem_TransportRequirements(t *testing.T) {
	base := Item{
		ID:           "browser-use",
		Name:         "Browser Use",
		SourceURL:    "https://example.com/browser-use",
		TrustTier:    TrustOfficial,
		ReviewStatus: ReviewApproved,
	}

	stdio := base
	stdio.Transport = TransportStdio
	if err := ValidateItem(stdio); err == nil {
		t.Fatal("expected stdio item without command to fail")
	}

	sse := base
	sse.Transport = TransportSSE
	sse.URL = "https://mcp.example.com/sse"
	if err := ValidateItem(sse); err != nil {
		t.Fatalf("sse item: %v", err)
	}
}
