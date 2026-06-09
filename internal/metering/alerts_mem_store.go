package metering

import (
	"context"
	"sync"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// MemAlertRuleStore is an in-memory AlertRuleStore for development and testing.
type MemAlertRuleStore struct {
	mu    sync.RWMutex
	rules []*AlertRule
}

func NewMemAlertRuleStore() *MemAlertRuleStore {
	return &MemAlertRuleStore{}
}

func (m *MemAlertRuleStore) Create(_ context.Context, rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
	return nil
}

func (m *MemAlertRuleStore) Get(_ context.Context, tenantID, ruleID string) (*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.rules {
		if r.TenantID == tenantID && r.ID == ruleID {
			return r, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *MemAlertRuleStore) List(_ context.Context, tenantID string) ([]*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*AlertRule
	for _, r := range m.rules {
		if r.TenantID == tenantID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MemAlertRuleStore) Update(_ context.Context, rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.rules {
		if r.TenantID == rule.TenantID && r.ID == rule.ID {
			m.rules[i] = rule
			return nil
		}
	}
	return store.ErrNotFound
}

func (m *MemAlertRuleStore) Delete(_ context.Context, tenantID, ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.rules {
		if r.TenantID == tenantID && r.ID == ruleID {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MemAlertRuleStore) ListAllEnabled(_ context.Context) ([]*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*AlertRule
	for _, r := range m.rules {
		if r.Enabled {
			result = append(result, r)
		}
	}
	return result, nil
}

// MemAlertEventStore is an in-memory AlertEventStore for development and testing.
type MemAlertEventStore struct {
	mu     sync.RWMutex
	events []*AlertEvent
}

func NewMemAlertEventStore() *MemAlertEventStore {
	return &MemAlertEventStore{}
}

func (m *MemAlertEventStore) Record(_ context.Context, event *AlertEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *MemAlertEventStore) ListByTenant(ctx context.Context, tenantID string, limit int) ([]*AlertEvent, error) {
	return m.ListByTenantPage(ctx, tenantID, limit, 0)
}

func (m *MemAlertEventStore) ListByTenantPage(_ context.Context, tenantID string, limit, offset int) ([]*AlertEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if offset < 0 {
		offset = 0
	}
	var result []*AlertEvent
	skipped := 0
	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].TenantID == tenantID {
			if skipped < offset {
				skipped++
				continue
			}
			result = append(result, m.events[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
