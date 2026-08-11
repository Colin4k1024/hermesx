package metering

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockUsageStore is a test double for UsageStore.
type mockUsageStore struct {
	mu      sync.Mutex
	batches [][]UsageRecord
}

func (m *mockUsageStore) BatchInsert(_ context.Context, records []UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches = append(m.batches, records)
	return nil
}

func (m *mockUsageStore) QueryByTenant(_ context.Context, _ string, _, _ time.Time, _ string) ([]UsageSummary, error) {
	return nil, nil
}

func (m *mockUsageStore) QueryBySession(_ context.Context, _, _ string) ([]UsageRecord, error) {
	return nil, nil
}

func (m *mockUsageStore) totalRecords() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, b := range m.batches {
		total += len(b)
	}
	return total
}

func TestUsageRecorder_FlushOnBufferFull(t *testing.T) {
	store := &mockUsageStore{}
	rec := NewUsageRecorder(store, WithBufferSize(5), WithFlushInterval(10*time.Second), WithChannelCap(100))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rec.Start(ctx)
		close(done)
	}()

	// Send exactly 5 records to trigger buffer flush.
	for i := 0; i < 5; i++ {
		rec.Record(UsageRecord{
			TenantID:    "t1",
			SessionID:   "s1",
			Model:       "gpt-4o",
			Provider:    "openai",
			InputTokens: 100,
		})
	}

	// Give flush goroutine time to process.
	time.Sleep(100 * time.Millisecond)

	if store.totalRecords() != 5 {
		t.Errorf("expected 5 records flushed, got %d", store.totalRecords())
	}

	cancel()
	<-done
}

func TestUsageRecorder_FlushOnTimer(t *testing.T) {
	store := &mockUsageStore{}
	rec := NewUsageRecorder(store, WithBufferSize(100), WithFlushInterval(50*time.Millisecond), WithChannelCap(100))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rec.Start(ctx)
		close(done)
	}()

	rec.Record(UsageRecord{TenantID: "t1", SessionID: "s1", Model: "gpt-4o", Provider: "openai"})

	// Wait for timer flush.
	time.Sleep(150 * time.Millisecond)

	if store.totalRecords() != 1 {
		t.Errorf("expected 1 record flushed on timer, got %d", store.totalRecords())
	}

	cancel()
	<-done
}

func TestUsageRecorder_DrainOnStop(t *testing.T) {
	store := &mockUsageStore{}
	rec := NewUsageRecorder(store, WithBufferSize(100), WithFlushInterval(10*time.Second), WithChannelCap(100))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rec.Start(ctx)
		close(done)
	}()

	// Enqueue 3 records (below buffer threshold, before timer).
	for i := 0; i < 3; i++ {
		rec.Record(UsageRecord{TenantID: "t1", SessionID: "s1", Model: "gpt-4o", Provider: "openai"})
	}

	// Give time for records to enter the channel.
	time.Sleep(20 * time.Millisecond)

	// Cancel context to trigger drain.
	cancel()
	<-done

	if store.totalRecords() != 3 {
		t.Errorf("expected 3 records drained on stop, got %d", store.totalRecords())
	}
}

func TestUsageRecorder_DropWhenFull(t *testing.T) {
	store := &mockUsageStore{}
	rec := NewUsageRecorder(store, WithBufferSize(100), WithFlushInterval(10*time.Second), WithChannelCap(2))

	// Don't start the recorder — channel should fill up.
	rec.Record(UsageRecord{TenantID: "t1", SessionID: "s1", Model: "gpt-4o", Provider: "openai"})
	rec.Record(UsageRecord{TenantID: "t1", SessionID: "s2", Model: "gpt-4o", Provider: "openai"})
	// This one should be dropped (channel capacity is 2).
	rec.Record(UsageRecord{TenantID: "t1", SessionID: "s3", Model: "gpt-4o", Provider: "openai"})

	if len(rec.ch) != 2 {
		t.Errorf("expected channel to have 2 items, got %d", len(rec.ch))
	}
}

func TestUsageRecorder_Stop(t *testing.T) {
	store := &mockUsageStore{}
	rec := NewUsageRecorder(store, WithBufferSize(10), WithFlushInterval(time.Second), WithChannelCap(100))

	// Enqueue a record.
	rec.Record(UsageRecord{TenantID: "t1", SessionID: "s1", Model: "gpt-4o", Provider: "openai"})

	// Stop should close channels without panic.
	rec.Stop()

	// Calling Stop again should be idempotent (once.Do).
	rec.Stop()
}

func TestLogAlertNotifier_Notify(t *testing.T) {
	n := &LogAlertNotifier{}
	event := &AlertEvent{
		TenantID:   "tenant-1",
		Metric:     MetricInputTokens,
		Threshold:  100,
		Current:    150,
		Percentage: 150.0,
	}
	// Should just log and return nil.
	if err := n.Notify(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithAlertInterval(t *testing.T) {
	checker := &AlertChecker{}
	WithAlertInterval(5 * time.Second)(checker)
	if checker.interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", checker.interval)
	}
}
