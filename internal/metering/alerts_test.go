package metering

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memAlertRuleStore struct {
	mu    sync.Mutex
	rules []*AlertRule
}

func (m *memAlertRuleStore) Create(_ context.Context, rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
	return nil
}

func (m *memAlertRuleStore) Get(_ context.Context, tenantID, ruleID string) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rules {
		if r.TenantID == tenantID && r.ID == ruleID {
			return r, nil
		}
	}
	return nil, nil
}

func (m *memAlertRuleStore) List(_ context.Context, tenantID string) ([]*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*AlertRule
	for _, r := range m.rules {
		if r.TenantID == tenantID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *memAlertRuleStore) Update(_ context.Context, rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.rules {
		if r.TenantID == rule.TenantID && r.ID == rule.ID {
			m.rules[i] = rule
			return nil
		}
	}
	return nil
}

func (m *memAlertRuleStore) Delete(_ context.Context, tenantID, ruleID string) error {
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

func (m *memAlertRuleStore) ListAllEnabled(_ context.Context) ([]*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*AlertRule
	for _, r := range m.rules {
		if r.Enabled {
			result = append(result, r)
		}
	}
	return result, nil
}

type memAlertEventStore struct {
	mu     sync.Mutex
	events []*AlertEvent
}

func (m *memAlertEventStore) Record(_ context.Context, event *AlertEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *memAlertEventStore) ListByTenant(_ context.Context, tenantID string, limit int) ([]*AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*AlertEvent
	for _, e := range m.events {
		if e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

type memUsageStore struct {
	summaries map[string][]UsageSummary
}

func (m *memUsageStore) BatchInsert(_ context.Context, _ []UsageRecord) error { return nil }
func (m *memUsageStore) QueryBySession(_ context.Context, _, _ string) ([]UsageRecord, error) {
	return nil, nil
}
func (m *memUsageStore) QueryByTenant(_ context.Context, tenantID string, _, _ time.Time, _ string) ([]UsageSummary, error) {
	return m.summaries[tenantID], nil
}

type captureNotifier struct {
	mu     sync.Mutex
	events []*AlertEvent
}

func (n *captureNotifier) Notify(_ context.Context, event *AlertEvent) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, event)
	return nil
}

func TestAlertChecker_ThresholdExceeded(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricTotalTokens, Threshold: 1000, Window: "daily", Enabled: true},
		},
	}
	events := &memAlertEventStore{}
	usage := &memUsageStore{
		summaries: map[string][]UsageSummary{
			"t1": {{Date: "2025-01-01", InputTokens: 600, OutputTokens: 500, CostUSD: 0.01}},
		},
	}
	notifier := &captureNotifier{}

	checker := NewAlertChecker(rules, events, usage, WithAlertNotifier(notifier))
	checker.tick(context.Background())

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.events) != 1 {
		t.Fatalf("expected 1 alert event, got %d", len(notifier.events))
	}
	ev := notifier.events[0]
	if ev.TenantID != "t1" || ev.Metric != MetricTotalTokens {
		t.Errorf("unexpected event: %+v", ev)
	}
	if ev.Current != 1100 {
		t.Errorf("current = %v, want 1100", ev.Current)
	}
	if ev.Percentage < 100 {
		t.Errorf("percentage = %v, want >= 100", ev.Percentage)
	}

	events.mu.Lock()
	defer events.mu.Unlock()
	if len(events.events) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(events.events))
	}
}

func TestAlertChecker_BelowThreshold(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricCostUSD, Threshold: 10.0, Window: "monthly", Enabled: true},
		},
	}
	events := &memAlertEventStore{}
	usage := &memUsageStore{
		summaries: map[string][]UsageSummary{
			"t1": {{Date: "2025-01-01", InputTokens: 100, OutputTokens: 50, CostUSD: 2.5}},
		},
	}
	notifier := &captureNotifier{}

	checker := NewAlertChecker(rules, events, usage, WithAlertNotifier(notifier))
	checker.tick(context.Background())

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.events) != 0 {
		t.Fatalf("expected 0 alert events, got %d", len(notifier.events))
	}
}

func TestAlertChecker_DeduplicatesWithinWindow(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricInputTokens, Threshold: 500, Window: "daily", Enabled: true},
		},
	}
	events := &memAlertEventStore{}
	usage := &memUsageStore{
		summaries: map[string][]UsageSummary{
			"t1": {{Date: "2025-01-01", InputTokens: 600, OutputTokens: 50, CostUSD: 0.01}},
		},
	}
	notifier := &captureNotifier{}

	checker := NewAlertChecker(rules, events, usage, WithAlertNotifier(notifier))

	checker.tick(context.Background())
	checker.tick(context.Background())
	checker.tick(context.Background())

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.events) != 1 {
		t.Fatalf("expected 1 deduplicated alert, got %d", len(notifier.events))
	}
}

func TestAlertChecker_DisabledRuleSkipped(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricTotalTokens, Threshold: 100, Window: "daily", Enabled: false},
		},
	}
	events := &memAlertEventStore{}
	usage := &memUsageStore{
		summaries: map[string][]UsageSummary{
			"t1": {{Date: "2025-01-01", InputTokens: 600, OutputTokens: 500, CostUSD: 1.0}},
		},
	}
	notifier := &captureNotifier{}

	checker := NewAlertChecker(rules, events, usage, WithAlertNotifier(notifier))
	checker.tick(context.Background())

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.events) != 0 {
		t.Fatalf("expected 0 alerts for disabled rule, got %d", len(notifier.events))
	}
}

func TestWindowBounds_Daily(t *testing.T) {
	now := time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC)
	from, to := windowBounds("daily", now)

	wantFrom := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2025, 3, 16, 0, 0, 0, 0, time.UTC)

	if !from.Equal(wantFrom) {
		t.Errorf("from = %v, want %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Errorf("to = %v, want %v", to, wantTo)
	}
}

func TestWindowBounds_Monthly(t *testing.T) {
	now := time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC)
	from, to := windowBounds("monthly", now)

	wantFrom := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	if !from.Equal(wantFrom) {
		t.Errorf("from = %v, want %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Errorf("to = %v, want %v", to, wantTo)
	}
}
