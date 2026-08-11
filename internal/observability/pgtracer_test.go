package observability

import (
	"context"
	"testing"
)

func TestPGQueryStart_ReturnsContextWithState(t *testing.T) {
	ctx := context.Background()
	newCtx := PGQueryStart(ctx, "SELECT")

	if newCtx == nil {
		t.Fatal("PGQueryStart returned nil context")
	}

	// Verify state is stored in context
	v := newCtx.Value(pgQueryStartKey{})
	if v == nil {
		t.Fatal("PGQueryStart did not store state in context")
	}

	state, ok := v.(*pgQueryState)
	if !ok {
		t.Fatal("stored value is not *pgQueryState")
	}
	if state.operation != "SELECT" {
		t.Fatalf("state.operation = %q, want %q", state.operation, "SELECT")
	}
	if state.start.IsZero() {
		t.Fatal("state.start should not be zero")
	}
}

func TestPGQueryEnd_WithValidContext(t *testing.T) {
	ctx := context.Background()
	ctx = PGQueryStart(ctx, "INSERT")

	// Should not panic
	PGQueryEnd(ctx)
}

func TestPGQueryEnd_WithNoState(t *testing.T) {
	ctx := context.Background()

	// Should be a no-op and not panic
	PGQueryEnd(ctx)
}

func TestPGQueryEnd_WithNilValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), pgQueryStartKey{}, nil)

	// Should be a no-op and not panic
	PGQueryEnd(ctx)
}

func TestSqlPrefix_ShortString(t *testing.T) {
	input := "SELECT * FROM users"
	result := sqlPrefix(input)
	if result != input {
		t.Fatalf("sqlPrefix(%q) = %q, want %q", input, result, input)
	}
}

func TestSqlPrefix_LongString(t *testing.T) {
	input := "SELECT id, name, email, created_at, updated_at FROM users WHERE active = true AND role = 'admin'"
	result := sqlPrefix(input)
	expected := input[:40]
	if result != expected {
		t.Fatalf("sqlPrefix returned %q (len %d), want %q (len 40)", result, len(result), expected)
	}
	if len(result) != 40 {
		t.Fatalf("sqlPrefix result length = %d, want 40", len(result))
	}
}

func TestSqlPrefix_Exactly40Chars(t *testing.T) {
	input := "1234567890123456789012345678901234567890" // exactly 40 chars
	if len(input) != 40 {
		t.Fatalf("test setup: input length = %d, want 40", len(input))
	}
	result := sqlPrefix(input)
	if result != input {
		t.Fatalf("sqlPrefix(%q) = %q, want %q", input, result, input)
	}
}

func TestSqlPrefix_EmptyString(t *testing.T) {
	result := sqlPrefix("")
	if result != "" {
		t.Fatalf("sqlPrefix(\"\") = %q, want empty string", result)
	}
}

func TestPGQueryStart_DifferentOperations(t *testing.T) {
	operations := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "batch_insert"}
	for _, op := range operations {
		ctx := PGQueryStart(context.Background(), op)
		state := ctx.Value(pgQueryStartKey{}).(*pgQueryState)
		if state.operation != op {
			t.Errorf("operation = %q, want %q", state.operation, op)
		}
	}
}
