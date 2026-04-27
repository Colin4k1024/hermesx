package agent

import (
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
)

func TestValidateToolCalls_Valid(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path": "/tmp/test.txt"}`}},
		{ID: "2", Function: llm.FunctionCall{Name: "terminal", Arguments: `{"command": "ls"}`}},
	}

	valid, errors := ValidateToolCalls(calls)
	if len(valid) != 2 {
		t.Errorf("Expected 2 valid calls, got %d", len(valid))
	}
	if len(errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(errors))
	}
}

func TestValidateToolCalls_Truncated(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path": "/tmp/test.txt"}`}},
		{ID: "2", Function: llm.FunctionCall{Name: "write_file", Arguments: `{"path": "/tmp/out.txt", "content": "hello`}},
	}

	valid, errors := ValidateToolCalls(calls)
	if len(valid) != 1 {
		t.Errorf("Expected 1 valid call, got %d", len(valid))
	}
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
	if errors[0].ToolCall.ID != "2" {
		t.Errorf("Expected error for call ID '2', got '%s'", errors[0].ToolCall.ID)
	}
}

func TestValidateToolCalls_EmptyArgs(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Function: llm.FunctionCall{Name: "todo", Arguments: ""}},
	}

	valid, errors := ValidateToolCalls(calls)
	if len(valid) != 1 {
		t.Errorf("Expected 1 valid (empty args treated as {}), got %d valid, %d errors", len(valid), len(errors))
	}
}

func TestValidateToolCalls_AllInvalid(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Function: llm.FunctionCall{Name: "a", Arguments: `{`}},
		{ID: "2", Function: llm.FunctionCall{Name: "b", Arguments: `{"key": [`}},
	}

	valid, errors := ValidateToolCalls(calls)
	if len(valid) != 0 {
		t.Errorf("Expected 0 valid, got %d", len(valid))
	}
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}
}
