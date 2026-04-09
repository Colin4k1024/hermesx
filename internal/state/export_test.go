package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportSessionJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "export_test.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	// Create session with messages
	db.CreateSession("export-session", "cli", "gpt-4", "")
	db.SetSessionTitle("export-session", "Test Export")
	db.AppendMessage("export-session", "user", "Hello there", "", "", nil, "")
	db.AppendMessage("export-session", "assistant", "Hi! How can I help?", "", "", nil, "thinking about response")
	db.AppendMessage("export-session", "tool", "file contents here", "tc_1", "read_file", nil, "")

	// Export to JSON
	outputPath := filepath.Join(tmpDir, "export.json")
	err = ExportSessionJSON(db, "export-session", outputPath)
	if err != nil {
		t.Fatalf("ExportSessionJSON failed: %v", err)
	}

	// Read and parse the exported file
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read export: %v", err)
	}

	var exported ExportedSession
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}

	if exported.SessionID != "export-session" {
		t.Errorf("Expected session ID 'export-session', got '%s'", exported.SessionID)
	}
	if exported.Title != "Test Export" {
		t.Errorf("Expected title 'Test Export', got '%s'", exported.Title)
	}
	if len(exported.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(exported.Messages))
	}
	if exported.Messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", exported.Messages[0].Role)
	}
	if exported.Messages[0].Content != "Hello there" {
		t.Errorf("Expected first message content 'Hello there', got '%s'", exported.Messages[0].Content)
	}
	if exported.ExportedAt == "" {
		t.Error("Expected non-empty ExportedAt")
	}
}

func TestExportSessionJSON_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "export_test2.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	err = ExportSessionJSON(db, "nonexistent", filepath.Join(tmpDir, "out.json"))
	if err == nil {
		t.Error("Expected error for nonexistent session")
	}
}

func TestExportSessionMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "export_md_test.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	db.CreateSession("md-session", "telegram", "claude-3", "")
	db.SetSessionTitle("md-session", "Markdown Export Test")
	db.AppendMessage("md-session", "user", "What is 2+2?", "", "", nil, "")
	db.AppendMessage("md-session", "assistant", "The answer is 4.", "", "", nil, "Let me calculate")
	db.AppendMessage("md-session", "tool", "result: 4", "tc_1", "calculator", nil, "")
	db.AppendMessage("md-session", "system", "System prompt", "", "", nil, "")

	outputPath := filepath.Join(tmpDir, "export.md")
	err = ExportSessionMarkdown(db, "md-session", outputPath)
	if err != nil {
		t.Fatalf("ExportSessionMarkdown failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read export: %v", err)
	}

	content := string(data)

	// Check headers
	if !strings.Contains(content, "# Session: Markdown Export Test") {
		t.Error("Expected session title in markdown")
	}
	if !strings.Contains(content, "**Session ID:**") {
		t.Error("Expected session ID in markdown")
	}

	// Check user message
	if !strings.Contains(content, "## User") {
		t.Error("Expected '## User' heading")
	}
	if !strings.Contains(content, "What is 2+2?") {
		t.Error("Expected user message content")
	}

	// Check assistant message
	if !strings.Contains(content, "## Assistant") {
		t.Error("Expected '## Assistant' heading")
	}
	if !strings.Contains(content, "The answer is 4.") {
		t.Error("Expected assistant message content")
	}

	// Check tool result
	if !strings.Contains(content, "### Tool Result (calculator)") {
		t.Error("Expected tool result heading")
	}
	if !strings.Contains(content, "result: 4") {
		t.Error("Expected tool result content")
	}

	// System messages should be omitted
	if strings.Contains(content, "System prompt") {
		t.Error("System messages should be omitted from markdown export")
	}
}

func TestExportSessionMarkdown_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "export_md_test2.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	err = ExportSessionMarkdown(db, "nonexistent", filepath.Join(tmpDir, "out.md"))
	if err == nil {
		t.Error("Expected error for nonexistent session")
	}
}

// --- Helper function tests ---

func TestStringVal(t *testing.T) {
	m := map[string]any{
		"key": "value",
		"num": 42,
	}

	if stringVal(m, "key") != "value" {
		t.Error("Expected 'value'")
	}
	if stringVal(m, "num") != "" {
		t.Error("Expected empty for non-string")
	}
	if stringVal(m, "missing") != "" {
		t.Error("Expected empty for missing key")
	}
}

func TestInt64Val(t *testing.T) {
	m := map[string]any{
		"int":     int(42),
		"int64":   int64(100),
		"float64": float64(200),
		"string":  "not a number",
	}

	if int64Val(m, "int") != 42 {
		t.Errorf("Expected 42, got %d", int64Val(m, "int"))
	}
	if int64Val(m, "int64") != 100 {
		t.Errorf("Expected 100, got %d", int64Val(m, "int64"))
	}
	if int64Val(m, "float64") != 200 {
		t.Errorf("Expected 200, got %d", int64Val(m, "float64"))
	}
	if int64Val(m, "string") != 0 {
		t.Errorf("Expected 0 for string type, got %d", int64Val(m, "string"))
	}
	if int64Val(m, "missing") != 0 {
		t.Errorf("Expected 0 for missing key, got %d", int64Val(m, "missing"))
	}
}

func TestBuildExportedSession_UntitledSession(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "untitled_test.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	// Session without a title
	db.CreateSession("no-title-session", "cli", "model", "")
	db.AppendMessage("no-title-session", "user", "Hello", "", "", nil, "")

	exported, err := buildExportedSession(db, "no-title-session")
	if err != nil {
		t.Fatalf("buildExportedSession failed: %v", err)
	}
	if exported.Title != "Untitled Session" {
		t.Errorf("Expected 'Untitled Session', got '%s'", exported.Title)
	}
}

func TestExportedSession_TokensInExport(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dbPath := filepath.Join(tmpDir, "tokens_test.db")
	db, err := NewSessionDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	db.CreateSession("token-session", "cli", "model", "")
	db.UpdateTokenCounts("token-session", 1000, 500, 0, 0, 0)

	exported, err := buildExportedSession(db, "token-session")
	if err != nil {
		t.Fatalf("buildExportedSession failed: %v", err)
	}
	if exported.Tokens.Input != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", exported.Tokens.Input)
	}
	if exported.Tokens.Output != 500 {
		t.Errorf("Expected 500 output tokens, got %d", exported.Tokens.Output)
	}
}
