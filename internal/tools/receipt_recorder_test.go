package tools

import (
	"context"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestReceiptRecorder_Record(t *testing.T) {
	store := &mockExecutionReceiptStore{}
	recorder := NewReceiptRecorder(store)
	if recorder == nil {
		t.Fatal("expected non-nil recorder")
	}

	tctx := &ToolContext{
		TenantID:  "test-tenant",
		SessionID: "test-session",
		UserID:    "test-user",
	}

	recorder.Record(context.Background(), "test_tool", map[string]any{"arg1": "value1"}, tctx, "result")

	if len(store.receipts) != 1 {
		t.Errorf("expected 1 receipt, got %d", len(store.receipts))
	}
}

func TestReceiptRecorder_RecordWithDuration(t *testing.T) {
	store := &mockExecutionReceiptStore{}
	recorder := NewReceiptRecorder(store)
	if recorder == nil {
		t.Fatal("expected non-nil recorder")
	}

	tctx := &ToolContext{
		TenantID:  "test-tenant",
		SessionID: "test-session",
		UserID:    "test-user",
	}

	recorder.RecordWithDuration(context.Background(), "test_tool", map[string]any{"arg1": "value1"}, tctx, "result", 100)

	if len(store.receipts) != 1 {
		t.Errorf("expected 1 receipt, got %d", len(store.receipts))
	}

	if store.receipts[0].DurationMs != 100 {
		t.Errorf("expected duration_ms 100, got %d", store.receipts[0].DurationMs)
	}
}

func TestReceiptRecorder_CheckIdempotency(t *testing.T) {
	store := &mockExecutionReceiptStore{}
	recorder := NewReceiptRecorder(store)
	if recorder == nil {
		t.Fatal("expected non-nil recorder")
	}

	receipt, found := recorder.CheckIdempotency(context.Background(), "test-tenant", "idempotency-id")
	if !found {
		t.Error("expected found")
	}
	if receipt != nil {
		t.Error("expected nil receipt")
	}
}

func TestReceiptRecorder_NilRecorder(t *testing.T) {
	var recorder *ReceiptRecorder

	recorder.Record(context.Background(), "test_tool", nil, nil, "result")
}

func TestReceiptRecorder_NilStore(t *testing.T) {
	recorder := &ReceiptRecorder{store: nil}

	tctx := &ToolContext{
		TenantID:  "test-tenant",
		SessionID: "test-session",
		UserID:    "test-user",
	}

	recorder.Record(context.Background(), "test_tool", nil, tctx, "result")
}

func TestReceiptRecorder_EmptyTenantID(t *testing.T) {
	store := &mockExecutionReceiptStore{}
	recorder := NewReceiptRecorder(store)
	if recorder == nil {
		t.Fatal("expected non-nil recorder")
	}

	tctx := &ToolContext{
		TenantID:  "",
		SessionID: "test-session",
		UserID:    "test-user",
	}

	recorder.Record(context.Background(), "test_tool", nil, tctx, "result")

	if len(store.receipts) != 0 {
		t.Errorf("expected 0 receipts, got %d", len(store.receipts))
	}
}

type mockExecutionReceiptStore struct {
	receipts []*store.ExecutionReceipt
}

func (m *mockExecutionReceiptStore) Create(ctx context.Context, receipt *store.ExecutionReceipt) error {
	m.receipts = append(m.receipts, receipt)
	return nil
}

func (m *mockExecutionReceiptStore) Get(ctx context.Context, tenantID, id string) (*store.ExecutionReceipt, error) {
	return nil, nil
}

func (m *mockExecutionReceiptStore) List(ctx context.Context, tenantID string, opts store.ReceiptListOptions) ([]*store.ExecutionReceipt, int, error) {
	return nil, 0, nil
}

func (m *mockExecutionReceiptStore) GetByIdempotencyID(ctx context.Context, tenantID, idempotencyID string) (*store.ExecutionReceipt, error) {
	return nil, nil
}
