package metering

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// ---------------------------------------------------------------------------
// MemAlertRuleStore tests
// ---------------------------------------------------------------------------

func TestMemAlertRuleStore_CreateAndGet(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rule := &AlertRule{
		ID:        "rule-1",
		TenantID:  "tenant-a",
		Metric:    MetricInputTokens,
		Threshold: 1000,
		Window:    "daily",
		Enabled:   true,
	}

	if err := s.Create(ctx, rule); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	got, err := s.Get(ctx, "tenant-a", "rule-1")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.ID != rule.ID || got.TenantID != rule.TenantID || got.Metric != rule.Metric {
		t.Fatalf("Get returned wrong rule: got %+v, want %+v", got, rule)
	}
}

func TestMemAlertRuleStore_GetNotFound(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent-tenant", "nonexistent-rule")
	if err == nil {
		t.Fatal("Get: expected error for missing rule")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get: expected store.ErrNotFound, got %v", err)
	}
}

func TestMemAlertRuleStore_ListEmpty(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rules, err := s.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("List: expected 0 rules, got %d", len(rules))
	}
}

func TestMemAlertRuleStore_ListMultiple(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rules := []*AlertRule{
		{ID: "r1", TenantID: "tenant-a", Metric: MetricInputTokens, Threshold: 100, Window: "daily", Enabled: true},
		{ID: "r2", TenantID: "tenant-a", Metric: MetricOutputTokens, Threshold: 200, Window: "monthly", Enabled: false},
		{ID: "r3", TenantID: "tenant-b", Metric: MetricTotalTokens, Threshold: 300, Window: "daily", Enabled: true},
	}
	for _, r := range rules {
		if err := s.Create(ctx, r); err != nil {
			t.Fatalf("Create: unexpected error: %v", err)
		}
	}

	got, err := s.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List: expected 2 rules for tenant-a, got %d", len(got))
	}

	got, err = s.List(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List: expected 1 rule for tenant-b, got %d", len(got))
	}
}

func TestMemAlertRuleStore_Update(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rule := &AlertRule{
		ID:        "rule-1",
		TenantID:  "tenant-a",
		Metric:    MetricInputTokens,
		Threshold: 1000,
		Window:    "daily",
		Enabled:   true,
	}
	if err := s.Create(ctx, rule); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	updated := &AlertRule{
		ID:        "rule-1",
		TenantID:  "tenant-a",
		Metric:    MetricCostUSD,
		Threshold: 50.0,
		Window:    "monthly",
		Enabled:   false,
	}
	if err := s.Update(ctx, updated); err != nil {
		t.Fatalf("Update: unexpected error: %v", err)
	}

	got, err := s.Get(ctx, "tenant-a", "rule-1")
	if err != nil {
		t.Fatalf("Get after update: unexpected error: %v", err)
	}
	if got.Metric != MetricCostUSD {
		t.Fatalf("Update: expected Metric=%s, got %s", MetricCostUSD, got.Metric)
	}
	if got.Threshold != 50.0 {
		t.Fatalf("Update: expected Threshold=50.0, got %f", got.Threshold)
	}
	if got.Window != "monthly" {
		t.Fatalf("Update: expected Window=monthly, got %s", got.Window)
	}
	if got.Enabled {
		t.Fatal("Update: expected Enabled=false")
	}
}

func TestMemAlertRuleStore_UpdateNotFound(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rule := &AlertRule{
		ID:       "nonexistent",
		TenantID: "tenant-a",
	}
	err := s.Update(ctx, rule)
	if err == nil {
		t.Fatal("Update: expected error for missing rule")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Update: expected store.ErrNotFound, got %v", err)
	}
}

func TestMemAlertRuleStore_Delete(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rule := &AlertRule{
		ID:       "rule-1",
		TenantID: "tenant-a",
		Metric:   MetricInputTokens,
		Enabled:  true,
	}
	if err := s.Create(ctx, rule); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if err := s.Delete(ctx, "tenant-a", "rule-1"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	_, err := s.Get(ctx, "tenant-a", "rule-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get after Delete: expected store.ErrNotFound, got %v", err)
	}
}

func TestMemAlertRuleStore_DeleteNonexistent(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	// Delete on non-existent rule should not error
	err := s.Delete(ctx, "tenant-a", "nonexistent")
	if err != nil {
		t.Fatalf("Delete nonexistent: unexpected error: %v", err)
	}
}

func TestMemAlertRuleStore_ListAllEnabled(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	rules := []*AlertRule{
		{ID: "r1", TenantID: "tenant-a", Metric: MetricInputTokens, Enabled: true},
		{ID: "r2", TenantID: "tenant-a", Metric: MetricOutputTokens, Enabled: false},
		{ID: "r3", TenantID: "tenant-b", Metric: MetricTotalTokens, Enabled: true},
		{ID: "r4", TenantID: "tenant-b", Metric: MetricCostUSD, Enabled: false},
		{ID: "r5", TenantID: "tenant-c", Metric: MetricCostUSD, Enabled: true},
	}
	for _, r := range rules {
		if err := s.Create(ctx, r); err != nil {
			t.Fatalf("Create: unexpected error: %v", err)
		}
	}

	enabled, err := s.ListAllEnabled(ctx)
	if err != nil {
		t.Fatalf("ListAllEnabled: unexpected error: %v", err)
	}
	if len(enabled) != 3 {
		t.Fatalf("ListAllEnabled: expected 3 enabled rules, got %d", len(enabled))
	}
	for _, r := range enabled {
		if !r.Enabled {
			t.Fatalf("ListAllEnabled: returned disabled rule %s", r.ID)
		}
	}
}

func TestMemAlertRuleStore_ListAllEnabledEmpty(t *testing.T) {
	s := NewMemAlertRuleStore()
	ctx := context.Background()

	enabled, err := s.ListAllEnabled(ctx)
	if err != nil {
		t.Fatalf("ListAllEnabled: unexpected error: %v", err)
	}
	if len(enabled) != 0 {
		t.Fatalf("ListAllEnabled: expected 0, got %d", len(enabled))
	}
}

// ---------------------------------------------------------------------------
// MemAlertEventStore tests
// ---------------------------------------------------------------------------

func TestMemAlertEventStore_Record(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	event := &AlertEvent{
		ID:         "evt-1",
		TenantID:   "tenant-a",
		RuleID:     "rule-1",
		Metric:     MetricInputTokens,
		Threshold:  1000,
		Current:    1200,
		Percentage: 120.0,
		FiredAt:    time.Now(),
	}

	if err := s.Record(ctx, event); err != nil {
		t.Fatalf("Record: unexpected error: %v", err)
	}

	events, err := s.ListByTenant(ctx, "tenant-a", 10)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ListByTenant: expected 1 event, got %d", len(events))
	}
	if events[0].ID != "evt-1" {
		t.Fatalf("ListByTenant: expected event ID evt-1, got %s", events[0].ID)
	}
}

func TestMemAlertEventStore_ListByTenantEmpty(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	events, err := s.ListByTenant(ctx, "tenant-a", 10)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("ListByTenant: expected 0 events, got %d", len(events))
	}
}

func TestMemAlertEventStore_ListByTenantFilters(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	events := []*AlertEvent{
		{ID: "e1", TenantID: "tenant-a", RuleID: "r1", FiredAt: time.Now()},
		{ID: "e2", TenantID: "tenant-b", RuleID: "r2", FiredAt: time.Now()},
		{ID: "e3", TenantID: "tenant-a", RuleID: "r1", FiredAt: time.Now()},
	}
	for _, e := range events {
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: unexpected error: %v", err)
		}
	}

	got, err := s.ListByTenant(ctx, "tenant-a", 10)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByTenant: expected 2 events for tenant-a, got %d", len(got))
	}

	got, err = s.ListByTenant(ctx, "tenant-b", 10)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListByTenant: expected 1 event for tenant-b, got %d", len(got))
	}
}

func TestMemAlertEventStore_ListByTenantNewestFirst(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	// Record events in order e1, e2, e3
	for i, id := range []string{"e1", "e2", "e3"} {
		e := &AlertEvent{
			ID:       id,
			TenantID: "tenant-a",
			RuleID:   "r1",
			FiredAt:  time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: unexpected error: %v", err)
		}
	}

	got, err := s.ListByTenant(ctx, "tenant-a", 10)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
	// Newest first means e3, e2, e1
	if got[0].ID != "e3" || got[1].ID != "e2" || got[2].ID != "e1" {
		t.Fatalf("expected newest-first order [e3,e2,e1], got [%s,%s,%s]", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestMemAlertEventStore_ListByTenantLimit(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e := &AlertEvent{
			ID:       "e" + string(rune('0'+i)),
			TenantID: "tenant-a",
			RuleID:   "r1",
			FiredAt:  time.Now(),
		}
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: unexpected error: %v", err)
		}
	}

	got, err := s.ListByTenant(ctx, "tenant-a", 3)
	if err != nil {
		t.Fatalf("ListByTenant: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events with limit=3, got %d", len(got))
	}
}

func TestMemAlertEventStore_ListByTenantPageOffset(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	// Record 5 events for tenant-a
	ids := []string{"e1", "e2", "e3", "e4", "e5"}
	for _, id := range ids {
		e := &AlertEvent{
			ID:       id,
			TenantID: "tenant-a",
			RuleID:   "r1",
			FiredAt:  time.Now(),
		}
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: unexpected error: %v", err)
		}
	}

	// Page 1: offset=0, limit=2 -> newest first: e5, e4
	page1, err := s.ListByTenantPage(ctx, "tenant-a", 2, 0)
	if err != nil {
		t.Fatalf("ListByTenantPage: unexpected error: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1: expected 2 events, got %d", len(page1))
	}
	if page1[0].ID != "e5" || page1[1].ID != "e4" {
		t.Fatalf("page1: expected [e5,e4], got [%s,%s]", page1[0].ID, page1[1].ID)
	}

	// Page 2: offset=2, limit=2 -> e3, e2
	page2, err := s.ListByTenantPage(ctx, "tenant-a", 2, 2)
	if err != nil {
		t.Fatalf("ListByTenantPage: unexpected error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2: expected 2 events, got %d", len(page2))
	}
	if page2[0].ID != "e3" || page2[1].ID != "e2" {
		t.Fatalf("page2: expected [e3,e2], got [%s,%s]", page2[0].ID, page2[1].ID)
	}

	// Page 3: offset=4, limit=2 -> e1
	page3, err := s.ListByTenantPage(ctx, "tenant-a", 2, 4)
	if err != nil {
		t.Fatalf("ListByTenantPage: unexpected error: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page3: expected 1 event, got %d", len(page3))
	}
	if page3[0].ID != "e1" {
		t.Fatalf("page3: expected [e1], got [%s]", page3[0].ID)
	}
}

func TestMemAlertEventStore_ListByTenantPageNegativeOffset(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	e := &AlertEvent{ID: "e1", TenantID: "tenant-a", RuleID: "r1", FiredAt: time.Now()}
	if err := s.Record(ctx, e); err != nil {
		t.Fatalf("Record: unexpected error: %v", err)
	}

	// Negative offset should be treated as 0
	got, err := s.ListByTenantPage(ctx, "tenant-a", 10, -5)
	if err != nil {
		t.Fatalf("ListByTenantPage: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event with negative offset, got %d", len(got))
	}
}

func TestMemAlertEventStore_ListByTenantPageZeroLimit(t *testing.T) {
	s := NewMemAlertEventStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := &AlertEvent{
			ID:       "e" + string(rune('a'+i)),
			TenantID: "tenant-a",
			RuleID:   "r1",
			FiredAt:  time.Now(),
		}
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: unexpected error: %v", err)
		}
	}

	// limit=0 means no limit according to the implementation
	got, err := s.ListByTenantPage(ctx, "tenant-a", 0, 0)
	if err != nil {
		t.Fatalf("ListByTenantPage: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected all 3 events with limit=0, got %d", len(got))
	}
}
