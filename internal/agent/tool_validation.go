package agent

import (
	"encoding/json"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// ToolCallError represents a validation error for a tool call.
type ToolCallError struct {
	ToolCall llm.ToolCall
	Reason   string
}

// ValidateToolCalls checks for truncated tool calls (unclosed JSON).
// Returns validated calls and errors for invalid ones.
func ValidateToolCalls(calls []llm.ToolCall) (valid []llm.ToolCall, errors []ToolCallError) {
	for _, tc := range calls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}

		if !json.Valid([]byte(args)) {
			errors = append(errors, ToolCallError{
				ToolCall: tc,
				Reason:   fmt.Sprintf("truncated JSON arguments for tool '%s': unclosed brackets or invalid syntax", tc.Function.Name),
			})
			continue
		}

		valid = append(valid, tc)
	}
	return
}
