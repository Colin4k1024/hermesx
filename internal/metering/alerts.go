package metering

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AlertMetric identifies what usage metric to monitor.
type AlertMetric string

const (
	MetricInputTokens  AlertMetric = "input_tokens"
	MetricOutputTokens AlertMetric = "output_tokens"
	MetricTotalTokens  AlertMetric = "total_tokens"
	MetricCostUSD      AlertMetric = "cost_usd"
)

// AlertRule defines a threshold-based usage alert for a tenant.
type AlertRule struct {
	ID        string      `json:"id"`
	TenantID  string      `json:"tenant_id"`
	Metric    AlertMetric `json:"metric"`
	Threshold float64     `json:"threshold"`
	Window    string      `json:"window"` // "daily", "monthly"
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// AlertEvent represents a triggered alert notification.
type AlertEvent struct {
	ID         string      `json:"id"`
	TenantID   string      `json:"tenant_id"`
	RuleID     string      `json:"rule_id"`
	Metric     AlertMetric `json:"metric"`
	Threshold  float64     `json:"threshold"`
	Current    float64     `json:"current"`
	Percentage float64     `json:"percentage"`
	FiredAt    time.Time   `json:"fired_at"`
}

// AlertRuleStore manages persistence for alert rules.
type AlertRuleStore interface {
	Create(ctx context.Context, rule *AlertRule) error
	Get(ctx context.Context, tenantID, ruleID string) (*AlertRule, error)
	List(ctx context.Context, tenantID string) ([]*AlertRule, error)
	Update(ctx context.Context, rule *AlertRule) error
	Delete(ctx context.Context, tenantID, ruleID string) error
	ListAllEnabled(ctx context.Context) ([]*AlertRule, error)
}

// AlertEventStore manages persistence for fired alert events.
type AlertEventStore interface {
	Record(ctx context.Context, event *AlertEvent) error
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]*AlertEvent, error)
}

// AlertNotifier delivers alert events to tenant administrators.
type AlertNotifier interface {
	Notify(ctx context.Context, event *AlertEvent) error
}

// LogAlertNotifier is a simple notifier that logs alerts via slog.
type LogAlertNotifier struct{}

func (n *LogAlertNotifier) Notify(_ context.Context, event *AlertEvent) error {
	slog.Warn("usage alert triggered",
		"tenant_id", event.TenantID,
		"metric", event.Metric,
		"threshold", event.Threshold,
		"current", event.Current,
		"percentage", fmt.Sprintf("%.1f%%", event.Percentage))
	return nil
}

// AlertChecker evaluates alert rules against current usage on a schedule.
type AlertChecker struct {
	rules    AlertRuleStore
	events   AlertEventStore
	usage    UsageStore
	notifier AlertNotifier
	interval time.Duration

	mu            sync.Mutex
	lastFiredKeys map[string]time.Time // "tenantID:ruleID:window" → last fire time
}

type AlertCheckerOption func(*AlertChecker)

func WithAlertInterval(d time.Duration) AlertCheckerOption {
	return func(c *AlertChecker) { c.interval = d }
}

func WithAlertNotifier(n AlertNotifier) AlertCheckerOption {
	return func(c *AlertChecker) { c.notifier = n }
}

func NewAlertChecker(rules AlertRuleStore, events AlertEventStore, usage UsageStore, opts ...AlertCheckerOption) *AlertChecker {
	c := &AlertChecker{
		rules:         rules,
		events:        events,
		usage:         usage,
		notifier:      &LogAlertNotifier{},
		interval:      15 * time.Minute,
		lastFiredKeys: make(map[string]time.Time),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Run starts the background evaluation loop. Blocks until ctx is cancelled.
func (c *AlertChecker) Run(ctx context.Context) {
	slog.Info("usage_alert_checker_started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("usage_alert_checker_stopped")
			return
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

func (c *AlertChecker) tick(ctx context.Context) {
	rules, err := c.rules.ListAllEnabled(ctx)
	if err != nil {
		slog.Error("alert_checker_list_rules_failed", "error", err)
		return
	}

	for _, rule := range rules {
		if err := c.evaluate(ctx, rule); err != nil {
			slog.Warn("alert_checker_evaluate_failed", "rule_id", rule.ID, "tenant_id", rule.TenantID, "error", err)
		}
	}
}

func (c *AlertChecker) evaluate(ctx context.Context, rule *AlertRule) error {
	from, to := windowBounds(rule.Window, time.Now())

	summaries, err := c.usage.QueryByTenant(ctx, rule.TenantID, from, to, "day")
	if err != nil {
		return fmt.Errorf("query usage: %w", err)
	}

	var current float64
	for _, s := range summaries {
		switch rule.Metric {
		case MetricInputTokens:
			current += float64(s.InputTokens)
		case MetricOutputTokens:
			current += float64(s.OutputTokens)
		case MetricTotalTokens:
			current += float64(s.InputTokens + s.OutputTokens)
		case MetricCostUSD:
			current += s.CostUSD
		}
	}

	if current < rule.Threshold {
		return nil
	}

	// Deduplicate: don't fire same rule more than once per window.
	key := fmt.Sprintf("%s:%s:%s", rule.TenantID, rule.ID, rule.Window)
	c.mu.Lock()
	lastFired, seen := c.lastFiredKeys[key]
	if seen && lastFired.After(from) {
		c.mu.Unlock()
		return nil
	}
	c.lastFiredKeys[key] = time.Now()
	c.mu.Unlock()

	event := &AlertEvent{
		ID:         fmt.Sprintf("ae-%d", time.Now().UnixNano()),
		TenantID:   rule.TenantID,
		RuleID:     rule.ID,
		Metric:     rule.Metric,
		Threshold:  rule.Threshold,
		Current:    current,
		Percentage: (current / rule.Threshold) * 100,
		FiredAt:    time.Now(),
	}

	if c.events != nil {
		if err := c.events.Record(ctx, event); err != nil {
			slog.Warn("alert_event_record_failed", "error", err)
		}
	}

	return c.notifier.Notify(ctx, event)
}

func windowBounds(window string, now time.Time) (time.Time, time.Time) {
	switch window {
	case "monthly":
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to := from.AddDate(0, 1, 0)
		return from, to
	default: // "daily"
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to := from.AddDate(0, 0, 1)
		return from, to
	}
}
