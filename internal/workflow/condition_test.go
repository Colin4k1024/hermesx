package workflow

import (
	"encoding/json"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestLookupPath_SimpleKey(t *testing.T) {
	root := map[string]any{"key": "val"}
	got, ok := lookupPath(root, "key")
	if !ok {
		t.Fatal("expected ok=true for existing key")
	}
	if got != "val" {
		t.Fatalf("got %v, want %q", got, "val")
	}
}

func TestLookupPath_Nested(t *testing.T) {
	root := map[string]any{
		"a": map[string]any{
			"b": 42,
		},
	}
	got, ok := lookupPath(root, "a.b")
	if !ok {
		t.Fatal("expected ok=true for nested path")
	}
	if got != 42 {
		t.Fatalf("got %v, want 42", got)
	}
}

func TestLookupPath_MissingKey(t *testing.T) {
	root := map[string]any{"key": "val"}
	got, ok := lookupPath(root, "missing")
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestLookupPath_EmptyPathSegment(t *testing.T) {
	root := map[string]any{"a": map[string]any{"b": 1}}
	// path with a space-only segment
	got, ok := lookupPath(root, "a. .b")
	if ok {
		t.Fatal("expected ok=false for empty path segment")
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestLookupPath_DeepNested(t *testing.T) {
	root := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": "deep",
			},
		},
	}
	got, ok := lookupPath(root, "level1.level2.level3")
	if !ok {
		t.Fatal("expected ok=true for deep nested path")
	}
	if got != "deep" {
		t.Fatalf("got %v, want %q", got, "deep")
	}
}

func TestLookupPath_NonMapIntermediate(t *testing.T) {
	root := map[string]any{"a": "string_value"}
	_, ok := lookupPath(root, "a.b")
	if ok {
		t.Fatal("expected ok=false when intermediate is not a map")
	}
}

func TestNormalizedEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"float==int", float64(1.0), int(1), true},
		{"int==float", int(1), float64(1.0), true},
		{"string==string", "foo", "foo", true},
		{"string!=string", "foo", "bar", false},
		{"int==int", int(5), int(5), true},
		{"int!=int", int(5), int(6), false},
		{"float!=int", float64(1.5), int(1), false},
		{"nil==nil", nil, nil, true},
		{"bool==bool", true, true, true},
		{"bool!=bool", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizedEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("normalizedEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"int", int(42), 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"string_valid", "2.5", 2.5, true},
		{"string_invalid", "not-a-number", 0, false},
		{"json.Number_valid", json.Number("1.5"), 1.5, true},
		{"json.Number_invalid", json.Number("xyz"), 0, false},
		{"bool_unsupported", true, 0, false},
		{"nil_unsupported", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("toFloat(%v) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConditionMatches_NilCondition(t *testing.T) {
	result, err := conditionMatches(nil, map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("nil condition should always match")
	}
}

func TestConditionMatches_EqDefault(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "status", Op: "", Value: "done"}
	ctx := map[string]any{"status": "done"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for eq (default op)")
	}
}

func TestConditionMatches_EqExplicit(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "count", Op: "eq", Value: float64(5)}
	ctx := map[string]any{"count": int(5)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for eq with numeric normalization")
	}
}

func TestConditionMatches_EqMismatch(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "status", Op: "eq", Value: "done"}
	ctx := map[string]any{"status": "pending"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for eq when values differ")
	}
}

func TestConditionMatches_EqPathNotFound(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "missing", Op: "eq", Value: "x"}
	ctx := map[string]any{"other": "y"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match when path not found")
	}
}

func TestConditionMatches_Ne(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "status", Op: "ne", Value: "done"}
	ctx := map[string]any{"status": "pending"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for ne when values differ")
	}
}

func TestConditionMatches_NeEqual(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "status", Op: "ne", Value: "done"}
	ctx := map[string]any{"status": "done"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for ne when values are equal")
	}
}

func TestConditionMatches_NePathNotFound(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "missing", Op: "ne", Value: "x"}
	ctx := map[string]any{}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for ne when path not found")
	}
}

func TestConditionMatches_Exists(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "key", Op: "exists"}
	ctx := map[string]any{"key": "anything"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for exists when key is present")
	}
}

func TestConditionMatches_ExistsNotFound(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "missing", Op: "exists"}
	ctx := map[string]any{"other": 1}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for exists when key is absent")
	}
}

func TestConditionMatches_Gt(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "gt", Value: float64(10)}
	ctx := map[string]any{"score": int(15)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for gt when left > right")
	}
}

func TestConditionMatches_GtEqual(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "gt", Value: float64(10)}
	ctx := map[string]any{"score": int(10)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for gt when left == right")
	}
}

func TestConditionMatches_Gte(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "gte", Value: float64(10)}
	ctx := map[string]any{"score": int(10)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for gte when left == right")
	}
}

func TestConditionMatches_Lt(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "lt", Value: float64(10)}
	ctx := map[string]any{"score": int(5)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for lt when left < right")
	}
}

func TestConditionMatches_Lte(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "lte", Value: float64(10)}
	ctx := map[string]any{"score": int(10)}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for lte when left == right")
	}
}

func TestConditionMatches_GtNonNumericPath(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "name", Op: "gt", Value: float64(10)}
	ctx := map[string]any{"name": "alice"}
	_, err := conditionMatches(cond, ctx)
	if err == nil {
		t.Fatal("expected error for non-numeric path with gt")
	}
}

func TestConditionMatches_GtNonNumericValue(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "score", Op: "gt", Value: "not-a-number"}
	ctx := map[string]any{"score": int(10)}
	_, err := conditionMatches(cond, ctx)
	if err == nil {
		t.Fatal("expected error for non-numeric condition value with gt")
	}
}

func TestConditionMatches_GtPathNotFound(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "missing", Op: "gt", Value: float64(10)}
	ctx := map[string]any{}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for gt when path not found")
	}
}

func TestConditionMatches_ContainsString(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "msg", Op: "contains", Value: "hello"}
	ctx := map[string]any{"msg": "say hello world"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for contains with substring")
	}
}

func TestConditionMatches_ContainsStringMiss(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "msg", Op: "contains", Value: "goodbye"}
	ctx := map[string]any{"msg": "say hello world"}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for contains when substring is absent")
	}
}

func TestConditionMatches_ContainsSlice(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "tags", Op: "contains", Value: "go"}
	ctx := map[string]any{"tags": []any{"rust", "go", "python"}}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected match for contains in slice")
	}
}

func TestConditionMatches_ContainsSliceMiss(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "tags", Op: "contains", Value: "java"}
	ctx := map[string]any{"tags": []any{"rust", "go", "python"}}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for contains when item not in slice")
	}
}

func TestConditionMatches_ContainsUnsupportedType(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "num", Op: "contains", Value: "x"}
	ctx := map[string]any{"num": 42}
	_, err := conditionMatches(cond, ctx)
	if err == nil {
		t.Fatal("expected error for contains on unsupported type")
	}
}

func TestConditionMatches_ContainsPathNotFound(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "missing", Op: "contains", Value: "x"}
	ctx := map[string]any{}
	result, err := conditionMatches(cond, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected no match for contains when path not found")
	}
}

func TestConditionMatches_UnknownOp(t *testing.T) {
	cond := &store.WorkflowCondition{Path: "x", Op: "unknown_op", Value: "y"}
	ctx := map[string]any{"x": "y"}
	_, err := conditionMatches(cond, ctx)
	if err == nil {
		t.Fatal("expected error for unknown op")
	}
}
