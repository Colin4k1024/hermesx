package tools

import (
	"testing"
)

func TestExtractBase64FromOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name:    "valid base64 between markers",
			output:  "some stdout\n__FILE_BASE64_START__\nSGVsbG8gV29ybGQ=\n__FILE_BASE64_END__\n",
			wantErr: false,
		},
		{
			name:    "missing start marker",
			output:  "SGVsbG8gV29ybGQ=\n__FILE_BASE64_END__\n",
			wantErr: true,
		},
		{
			name:    "missing end marker",
			output:  "__FILE_BASE64_START__\nSGVsbG8gV29ybGQ=\n",
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			output:  "__FILE_BASE64_START__\nnot-valid-b64!!!\n__FILE_BASE64_END__\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := extractBase64FromOutput(tc.output)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(data) != "Hello World" {
				t.Errorf("expected 'Hello World', got %q", string(data))
			}
		})
	}
}

func TestBuildDocumentScript_HeadingAndParagraph(t *testing.T) {
	content := []any{
		map[string]any{"type": "heading", "text": "Introduction", "level": 1},
		map[string]any{"type": "paragraph", "text": "This is a test paragraph."},
	}

	script, err := buildDocumentScript("Test Doc", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Script should read data from stdin as JSON (not embed user text in source).
	if !scriptContains(script, `json.loads(sys.stdin.read())`) {
		t.Error("expected stdin JSON parsing in script")
	}
	if !scriptContains(script, `block["type"]`) {
		t.Error("expected block type check in script")
	}
	if !scriptContains(script, `doc.save(`) {
		t.Error("expected doc.save() call in script")
	}
}

func TestBuildDocumentScript_Table(t *testing.T) {
	content := []any{
		map[string]any{
			"type":    "table",
			"headers": []any{"Name", "Value"},
			"rows": []any{
				[]any{"A", "100"},
				[]any{"B", "200"},
			},
		},
	}

	script, err := buildDocumentScript("Table Doc", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Script should read data from stdin as JSON.
	if !scriptContains(script, "json.loads") {
		t.Error("expected stdin JSON parsing in script")
	}
	if !scriptContains(script, "Table Grid") {
		t.Error("expected Table Grid style in script")
	}
	if !scriptContains(script, "add_table") {
		t.Error("expected add_table call in script")
	}
}

func TestBuildDocumentScript_UnknownBlockType(t *testing.T) {
	content := []any{
		map[string]any{"type": "unknown_type"},
	}

	_, err := buildDocumentScript("Bad Doc", content)
	if err == nil {
		t.Fatal("expected error for unknown block type")
	}
}

func TestBuildPresentationScript_TitleAndContent(t *testing.T) {
	slides := []any{
		map[string]any{
			"layout":   "title",
			"title":    "Welcome",
			"subtitle": "Subtitle text",
		},
		map[string]any{
			"layout":  "content",
			"title":   "Slide 2",
			"content": []any{"Point 1", "Point 2", "Point 3"},
		},
	}

	script, err := buildPresentationScript("My Pres", slides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !scriptContains(script, "add_title_slide") {
		t.Error("expected add_title_slide in script")
	}
	if !scriptContains(script, "add_content_slide") {
		t.Error("expected add_content_slide in script")
	}
	if !scriptContains(script, "prs.save(") {
		t.Error("expected prs.save() in script")
	}
}

func TestBuildPresentationScript_UnknownLayout(t *testing.T) {
	slides := []any{
		map[string]any{"layout": "invalid_layout", "title": "Oops"},
	}

	_, err := buildPresentationScript("Bad Pres", slides)
	if err == nil {
		t.Fatal("expected error for unknown layout")
	}
}

func TestBuildPresentationScript_BlankSlide(t *testing.T) {
	slides := []any{
		map[string]any{"layout": "blank"},
	}

	script, err := buildPresentationScript("Blank Pres", slides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !scriptContains(script, "add_blank_slide") {
		t.Error("expected add_blank_slide in script")
	}
}

func scriptContains(script, substr string) bool {
	return len(script) > 0 && len(substr) > 0 && contains(script, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
