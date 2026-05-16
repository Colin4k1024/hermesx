package evolution

import (
	"strings"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// TaskClass is a category label for agent conversations.
type TaskClass = string

const (
	TaskClassCodingDebug   TaskClass = "coding.debug"
	TaskClassCodingFeature TaskClass = "coding.feature"
	TaskClassAnalysis      TaskClass = "analysis.code"
	TaskClassWritingDocs   TaskClass = "writing.docs"
	TaskClassGeneral       TaskClass = "general"
)

var debugKeywords = []string{
	"error", "fix", "bug", "issue", "crash", "fail", "broken",
	"debug", "traceback", "panic", "exception", "not working",
}

var featureKeywords = []string{
	"implement", "create", "add", "build", "feature", "new",
	"develop", "write code", "function", "method", "endpoint",
}

var analysisKeywords = []string{
	"explain", "review", "analyze", "what is", "how does",
	"understand", "describe", "summarize", "why",
}

var writingKeywords = []string{
	"document", "readme", "docs", "comment", "documentation", "changelog",
}

var codingToolNames = map[string]bool{
	"terminal": true, "run_command": true, "bash": true,
	"write_file": true, "create_file": true, "edit_file": true,
}

// DetectTaskClass infers the task class from the conversation's first user
// message and the list of tool names that were called.
func DetectTaskClass(messages []llm.Message, toolsUsed []string) TaskClass {
	firstMsg := strings.ToLower(firstUserMessage(messages))

	// Tool usage patterns take precedence.
	for _, t := range toolsUsed {
		if codingToolNames[t] {
			if containsAny(firstMsg, debugKeywords) {
				return TaskClassCodingDebug
			}
			return TaskClassCodingFeature
		}
	}

	// More specific intent keywords take precedence over generic ones.
	switch {
	case containsAny(firstMsg, writingKeywords):
		return TaskClassWritingDocs
	case containsAny(firstMsg, analysisKeywords):
		return TaskClassAnalysis
	case containsAny(firstMsg, debugKeywords):
		return TaskClassCodingDebug
	case containsAny(firstMsg, featureKeywords):
		return TaskClassCodingFeature
	default:
		return TaskClassGeneral
	}
}

func firstUserMessage(messages []llm.Message) string {
	for _, m := range messages {
		if m.Role == "user" {
			return m.Content
		}
	}
	return ""
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
