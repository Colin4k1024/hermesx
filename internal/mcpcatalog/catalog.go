package mcpcatalog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	TrustOfficial  = "official"
	TrustTrusted   = "trusted"
	TrustCommunity = "community"

	ReviewApproved = "approved"
	ReviewPending  = "pending"
	ReviewBlocked  = "blocked"

	TransportStdio = "stdio"
	TransportSSE   = "sse"
)

var ErrNotFound = errors.New("mcp catalog item not found")

// Item is a governed MCP server catalog entry.
type Item struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Version             string    `json:"version,omitempty"`
	Description         string    `json:"description,omitempty"`
	SourceURL           string    `json:"source_url"`
	TrustTier           string    `json:"trust_tier"`
	ReviewStatus        string    `json:"review_status"`
	Transport           string    `json:"transport"`
	Command             string    `json:"command,omitempty"`
	Args                []string  `json:"args,omitempty"`
	URL                 string    `json:"url,omitempty"`
	RequiredCredentials []string  `json:"required_credentials,omitempty"`
	Scopes              []string  `json:"scopes,omitempty"`
	EgressDomains       []string  `json:"egress_domains,omitempty"`
	SandboxRequired     bool      `json:"sandbox_required"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// TenantItemPolicy records whether a tenant may install or use a catalog item.
type TenantItemPolicy struct {
	TenantID  string    `json:"tenant_id"`
	ItemID    string    `json:"item_id"`
	Enabled   bool      `json:"enabled"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is the control-plane persistence boundary for MCP catalog state.
type Store interface {
	ListItems(ctx context.Context) ([]Item, error)
	GetItem(ctx context.Context, id string) (*Item, error)
	UpsertItem(ctx context.Context, item Item) (*Item, error)
	ListTenantPolicies(ctx context.Context, tenantID string) ([]TenantItemPolicy, error)
	SetTenantPolicy(ctx context.Context, policy TenantItemPolicy) (*TenantItemPolicy, error)
}

// MemoryStore is a deterministic in-memory implementation suitable for
// bootstrap, tests, and single-process deployments before DB persistence lands.
type MemoryStore struct {
	mu       sync.RWMutex
	items    map[string]Item
	policies map[string]TenantItemPolicy
	now      func() time.Time
}

func NewMemoryStore(seed ...Item) *MemoryStore {
	s := &MemoryStore{
		items:    make(map[string]Item),
		policies: make(map[string]TenantItemPolicy),
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, item := range seed {
		_, _ = s.UpsertItem(context.Background(), item)
	}
	return s
}

func (s *MemoryStore) ListItems(_ context.Context) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, cloneItem(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (s *MemoryStore) GetItem(_ context.Context, id string) (*Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := cloneItem(item)
	return &clone, nil
}

func (s *MemoryStore) UpsertItem(_ context.Context, item Item) (*Item, error) {
	if err := ValidateItem(item); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	existing, ok := s.items[item.ID]
	if ok {
		item.CreatedAt = existing.CreatedAt
	} else if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	s.items[item.ID] = cloneItem(item)
	clone := cloneItem(item)
	return &clone, nil
}

func (s *MemoryStore) ListTenantPolicies(_ context.Context, tenantID string) ([]TenantItemPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]TenantItemPolicy, 0)
	for _, policy := range s.policies {
		if policy.TenantID == tenantID {
			policies = append(policies, policy)
		}
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].ItemID < policies[j].ItemID })
	return policies, nil
}

func (s *MemoryStore) SetTenantPolicy(_ context.Context, policy TenantItemPolicy) (*TenantItemPolicy, error) {
	if policy.TenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if policy.ItemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[policy.ItemID]; !ok {
		return nil, ErrNotFound
	}
	policy.UpdatedAt = s.now()
	s.policies[tenantPolicyKey(policy.TenantID, policy.ItemID)] = policy
	clone := policy
	return &clone, nil
}

func ValidateItem(item Item) error {
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(item.SourceURL) == "" {
		return fmt.Errorf("source_url is required")
	}
	if !validEnum(item.TrustTier, TrustOfficial, TrustTrusted, TrustCommunity) {
		return fmt.Errorf("trust_tier must be one of: official, trusted, community")
	}
	if !validEnum(item.ReviewStatus, ReviewApproved, ReviewPending, ReviewBlocked) {
		return fmt.Errorf("review_status must be one of: approved, pending, blocked")
	}
	transport := item.Transport
	if transport == "" {
		transport = TransportStdio
	}
	if !validEnum(transport, TransportStdio, TransportSSE) {
		return fmt.Errorf("transport must be one of: stdio, sse")
	}
	if transport == TransportStdio && strings.TrimSpace(item.Command) == "" {
		return fmt.Errorf("command is required for stdio transport")
	}
	if transport == TransportSSE && strings.TrimSpace(item.URL) == "" {
		return fmt.Errorf("url is required for sse transport")
	}
	return nil
}

func tenantPolicyKey(tenantID, itemID string) string {
	return tenantID + "\x00" + itemID
}

func validEnum(value string, allowed ...string) bool {
	for _, v := range allowed {
		if value == v {
			return true
		}
	}
	return false
}

func cloneItem(item Item) Item {
	item.Args = append([]string(nil), item.Args...)
	item.RequiredCredentials = append([]string(nil), item.RequiredCredentials...)
	item.Scopes = append([]string(nil), item.Scopes...)
	item.EgressDomains = append([]string(nil), item.EgressDomains...)
	return item
}
