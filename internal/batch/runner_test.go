package batch

import (
	"strings"
	"testing"
)

func TestCountSucceeded(t *testing.T) {
	results := []BatchResult{
		{Prompt: "p1", Response: "r1", Error: ""},
		{Prompt: "p2", Response: "", Error: "failed"},
		{Prompt: "p3", Response: "r3", Error: ""},
	}
	if got := countSucceeded(results); got != 2 {
		t.Errorf("countSucceeded = %d, want 2", got)
	}
}

func TestCountSucceeded_Empty(t *testing.T) {
	if got := countSucceeded(nil); got != 0 {
		t.Errorf("countSucceeded(nil) = %d, want 0", got)
	}
}

func TestCountFailed(t *testing.T) {
	results := []BatchResult{
		{Error: ""},
		{Error: "some error"},
		{Error: "another error"},
	}
	if got := countFailed(results); got != 2 {
		t.Errorf("countFailed = %d, want 2", got)
	}
}

func TestSumTokens(t *testing.T) {
	results := []BatchResult{
		{Tokens: 10},
		{Tokens: 25},
		{Tokens: 5},
	}
	if got := sumTokens(results); got != 40 {
		t.Errorf("sumTokens = %d, want 40", got)
	}
}

func TestSumTokens_Empty(t *testing.T) {
	if got := sumTokens(nil); got != 0 {
		t.Errorf("sumTokens(nil) = %d, want 0", got)
	}
}

func TestTruncatePrompt_Short(t *testing.T) {
	s := "short prompt"
	got := truncatePrompt(s)
	if got != s {
		t.Errorf("truncatePrompt short: got %q, want %q", got, s)
	}
}

func TestTruncatePrompt_Long(t *testing.T) {
	s := strings.Repeat("x", 100)
	got := truncatePrompt(s)
	if len(got) > 80 {
		t.Errorf("truncatePrompt long: got length %d, want <= 80", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("truncatePrompt long: should end with ...")
	}
}

func TestTruncatePrompt_Exactly80(t *testing.T) {
	s := strings.Repeat("y", 80)
	got := truncatePrompt(s)
	if got != s {
		t.Errorf("truncatePrompt at boundary: got %q, want same", got)
	}
}
