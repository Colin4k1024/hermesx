package tools

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateSpreadsheet_BasicGeneration(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		TaskID:      "task-1",
		ObjectStore: store,
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Test Report",
		"sheets": []any{
			map[string]any{
				"name":    "Sales",
				"headers": []any{"Product", "Revenue", "Units"},
				"data": []any{
					[]any{"Widget A", 1000.50, 50},
					[]any{"Widget B", 2500.00, 120},
				},
			},
		},
	}, tctx)

	if !strings.Contains(result, `"success":true`) {
		t.Fatalf("expected success, got: %s", result)
	}
	if !strings.Contains(result, `"filename":"Test Report.xlsx"`) {
		t.Fatalf("expected filename in result, got: %s", result)
	}
	if !strings.Contains(result, `"sheets":1`) {
		t.Fatalf("expected sheets count in result, got: %s", result)
	}

	// Verify file was uploaded to ObjectStore
	key := "artifacts/tenant-1/task-1/Test Report.xlsx"
	data, err := store.GetObject(context.Background(), key)
	if err != nil {
		t.Fatalf("expected file in object store, got error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty file data in object store")
	}
	// Verify it starts with the XLSX magic bytes (PK zip signature)
	if len(data) < 4 || data[0] != 'P' || data[1] != 'K' {
		t.Fatalf("expected XLSX file (PK zip), got first bytes: %v", data[:min(4, len(data))])
	}
}

func TestGenerateSpreadsheet_MultipleSheets(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		TaskID:      "task-2",
		ObjectStore: store,
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Multi-Sheet Report",
		"sheets": []any{
			map[string]any{
				"name":    "Sheet1",
				"headers": []any{"A", "B"},
				"data":    []any{[]any{1, 2}},
			},
			map[string]any{
				"name":    "Sheet2",
				"headers": []any{"X", "Y", "Z"},
				"data":    []any{[]any{10, 20, 30}},
			},
		},
	}, tctx)

	if !strings.Contains(result, `"success":true`) {
		t.Fatalf("expected success, got: %s", result)
	}
	if !strings.Contains(result, `"sheets":2`) {
		t.Fatalf("expected 2 sheets, got: %s", result)
	}
}

func TestGenerateSpreadsheet_NoObjectStore(t *testing.T) {
	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Test",
		"sheets": []any{
			map[string]any{
				"name": "Sheet1",
				"data": []any{[]any{1}},
			},
		},
	}, nil)

	if !strings.Contains(result, `"error"`) {
		t.Fatalf("expected error when ObjectStore is nil, got: %s", result)
	}
}

func TestGenerateSpreadsheet_MissingTitle(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		ObjectStore: store,
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"sheets": []any{map[string]any{"name": "Sheet1"}},
	}, tctx)

	if !strings.Contains(result, `"error"`) {
		t.Fatalf("expected error for missing title, got: %s", result)
	}
}

func TestGenerateSpreadsheet_MissingSheets(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		ObjectStore: store,
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Test",
	}, tctx)

	if !strings.Contains(result, `"error"`) {
		t.Fatalf("expected error for missing sheets, got: %s", result)
	}
}

func TestGenerateSpreadsheet_EmptySheets(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		ObjectStore: store,
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title":  "Test",
		"sheets": []any{},
	}, tctx)

	if !strings.Contains(result, `"error"`) {
		t.Fatalf("expected error for empty sheets, got: %s", result)
	}
}

func TestGenerateSpreadsheet_RowLimitExceeded(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		ObjectStore: store,
	}

	// Create data exceeding MaxSpreadsheetRows
	rows := make([]any, MaxSpreadsheetRows+1)
	for i := range rows {
		rows[i] = []any{i, "data"}
	}

	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Big Data",
		"sheets": []any{
			map[string]any{
				"name": "Sheet1",
				"data": rows,
			},
		},
	}, tctx)

	if !strings.Contains(result, `"error"`) {
		t.Fatalf("expected error for exceeding row limit, got: %s", result)
	}
	if !strings.Contains(result, "exceeds limit") {
		t.Fatalf("expected 'exceeds limit' in error message, got: %s", result)
	}
}

func TestGenerateSpreadsheet_EmptyDataSheet(t *testing.T) {
	store := newFakeSkillObjectStore()
	tctx := &ToolContext{
		TenantID:    "tenant-1",
		TaskID:      "task-3",
		ObjectStore: store,
	}

	// Sheet with headers but no data
	result := handleGenerateSpreadsheet(context.Background(), map[string]any{
		"title": "Empty Sheet",
		"sheets": []any{
			map[string]any{
				"name":    "Headers Only",
				"headers": []any{"Col1", "Col2", "Col3"},
				"data":    []any{},
			},
		},
	}, tctx)

	if !strings.Contains(result, `"success":true`) {
		t.Fatalf("expected success for empty data sheet, got: %s", result)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"path/with/slashes", "path_with_slashes"},
		{"file:name*special", "file_name_special"},
		{"..parent", "_parent"},
		{"", "document"},
		{"  spaces  ", "spaces"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestBuildArtifactKey(t *testing.T) {
	key := buildArtifactKey("tenant-1", "task-1", "file.xlsx")
	if key != "artifacts/tenant-1/task-1/file.xlsx" {
		t.Errorf("unexpected key: %s", key)
	}

	// Test with empty tenant and task
	key = buildArtifactKey("", "", "file.xlsx")
	if !strings.HasPrefix(key, "artifacts/default/") {
		t.Errorf("expected default tenant in key, got: %s", key)
	}
	if !strings.HasSuffix(key, "/file.xlsx") {
		t.Errorf("expected filename suffix in key, got: %s", key)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
