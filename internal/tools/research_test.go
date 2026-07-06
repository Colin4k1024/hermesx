package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseDepth(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "explicit quick",
			args: map[string]any{"depth": "quick"},
			want: "quick",
		},
		{
			name: "explicit standard",
			args: map[string]any{"depth": "standard"},
			want: "standard",
		},
		{
			name: "explicit deep",
			args: map[string]any{"depth": "deep"},
			want: "deep",
		},
		{
			name: "uppercase is normalized",
			args: map[string]any{"depth": "QUICK"},
			want: "quick",
		},
		{
			name: "empty defaults to standard",
			args: map[string]any{"depth": ""},
			want: "standard",
		},
		{
			name: "missing defaults to standard",
			args: map[string]any{},
			want: "standard",
		},
		{
			name: "invalid value defaults to standard",
			args: map[string]any{"depth": "ultra"},
			want: "standard",
		},
		{
			name: "whitespace is trimmed",
			args: map[string]any{"depth": "  deep  "},
			want: "deep",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDepth(tc.args)
			if got != tc.want {
				t.Errorf("parseDepth() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "explicit markdown",
			args: map[string]any{"output_format": "markdown"},
			want: "markdown",
		},
		{
			name: "explicit docx",
			args: map[string]any{"output_format": "docx"},
			want: "docx",
		},
		{
			name: "uppercase is normalized",
			args: map[string]any{"output_format": "DOCX"},
			want: "docx",
		},
		{
			name: "empty defaults to markdown",
			args: map[string]any{"output_format": ""},
			want: "markdown",
		},
		{
			name: "missing defaults to markdown",
			args: map[string]any{},
			want: "markdown",
		},
		{
			name: "invalid value defaults to markdown",
			args: map[string]any{"output_format": "pdf"},
			want: "markdown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOutputFormat(tc.args)
			if got != tc.want {
				t.Errorf("parseOutputFormat() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseSources(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want int
	}{
		{
			name: "valid sources",
			args: map[string]any{"sources": []any{"https://example.com", "https://test.org"}},
			want: 2,
		},
		{
			name: "empty array",
			args: map[string]any{"sources": []any{}},
			want: 0,
		},
		{
			name: "nil",
			args: map[string]any{},
			want: 0,
		},
		{
			name: "filters empty strings",
			args: map[string]any{"sources": []any{"https://example.com", "", "  ", "https://test.org"}},
			want: 2,
		},
		{
			name: "non-string items filtered",
			args: map[string]any{"sources": []any{"https://example.com", 123, "https://test.org"}},
			want: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSources(tc.args)
			if len(got) != tc.want {
				t.Errorf("parseSources() returned %d items, want %d", len(got), tc.want)
			}
		})
	}
}

func TestParseFindings(t *testing.T) {
	tests := []struct {
		name string
		raw  []any
		want int
	}{
		{
			name: "valid findings",
			raw: []any{
				map[string]any{"content": "Finding 1", "source_url": "https://a.com", "confidence": "high"},
				map[string]any{"content": "Finding 2", "confidence": "low"},
			},
			want: 2,
		},
		{
			name: "empty content is skipped",
			raw: []any{
				map[string]any{"content": "Valid"},
				map[string]any{"content": ""},
				map[string]any{"content": "  "},
			},
			want: 1,
		},
		{
			name: "non-map items skipped",
			raw: []any{
				"not a map",
				map[string]any{"content": "Valid"},
			},
			want: 1,
		},
		{
			name: "invalid confidence ignored",
			raw: []any{
				map[string]any{"content": "Test", "confidence": "extreme"},
			},
			want: 1,
		},
		{
			name: "empty input",
			raw:  []any{},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseFindings(tc.raw)
			if len(got) != tc.want {
				t.Errorf("parseFindings() returned %d items, want %d", len(got), tc.want)
			}
		})
	}
}

func TestParseFindings_ConfidenceValues(t *testing.T) {
	raw := []any{
		map[string]any{"content": "A", "confidence": "high"},
		map[string]any{"content": "B", "confidence": "medium"},
		map[string]any{"content": "C", "confidence": "low"},
		map[string]any{"content": "D", "confidence": "INVALID"},
	}

	findings := parseFindings(raw)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}

	if findings[0].Confidence != "high" {
		t.Errorf("finding 0 confidence = %q, want %q", findings[0].Confidence, "high")
	}
	if findings[1].Confidence != "medium" {
		t.Errorf("finding 1 confidence = %q, want %q", findings[1].Confidence, "medium")
	}
	if findings[2].Confidence != "low" {
		t.Errorf("finding 2 confidence = %q, want %q", findings[2].Confidence, "low")
	}
	if findings[3].Confidence != "" {
		t.Errorf("finding 3 confidence = %q, want empty (invalid value)", findings[3].Confidence)
	}
}

func TestGenerateResearchPlan_QuickDepth(t *testing.T) {
	result := generateResearchPlan("quantum computing", "quick", nil, "markdown")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["status"] != "plan_generated" {
		t.Errorf("status = %v, want plan_generated", parsed["status"])
	}

	plan, ok := parsed["plan"].(map[string]any)
	if !ok {
		t.Fatal("plan is not a map")
	}

	if plan["query"] != "quantum computing" {
		t.Errorf("plan.query = %v, want 'quantum computing'", plan["query"])
	}
	if plan["depth"] != "quick" {
		t.Errorf("plan.depth = %v, want 'quick'", plan["depth"])
	}
	if plan["output_format"] != "markdown" {
		t.Errorf("plan.output_format = %v, want 'markdown'", plan["output_format"])
	}

	subs, ok := plan["sub_questions"].([]any)
	if !ok {
		t.Fatal("sub_questions is not an array")
	}
	if len(subs) != 3 {
		t.Errorf("quick depth should have 3 sub_questions, got %d", len(subs))
	}

	if parsed["sub_question_count"] != float64(3) {
		t.Errorf("sub_question_count = %v, want 3", parsed["sub_question_count"])
	}
}

func TestGenerateResearchPlan_StandardDepth(t *testing.T) {
	result := generateResearchPlan("machine learning", "standard", nil, "markdown")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	plan := parsed["plan"].(map[string]any)
	subs := plan["sub_questions"].([]any)
	if len(subs) != 6 {
		t.Errorf("standard depth should have 6 sub_questions, got %d", len(subs))
	}
}

func TestGenerateResearchPlan_DeepDepth(t *testing.T) {
	result := generateResearchPlan("climate change", "deep", nil, "markdown")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	plan := parsed["plan"].(map[string]any)
	subs := plan["sub_questions"].([]any)
	if len(subs) != 10 {
		t.Errorf("deep depth should have 10 sub_questions, got %d", len(subs))
	}
}

func TestGenerateResearchPlan_WithSources(t *testing.T) {
	sources := []string{"https://example.com", "https://test.org"}
	result := generateResearchPlan("AI safety", "standard", sources, "docx")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	plan := parsed["plan"].(map[string]any)
	planSources, ok := plan["sources"].([]any)
	if !ok {
		t.Fatal("plan.sources is not an array")
	}
	if len(planSources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(planSources))
	}
	if plan["output_format"] != "docx" {
		t.Errorf("plan.output_format = %v, want 'docx'", plan["output_format"])
	}
}

func TestGenerateResearchPlan_SubQuestionStructure(t *testing.T) {
	result := generateResearchPlan("blockchain", "quick", nil, "markdown")

	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)

	plan := parsed["plan"].(map[string]any)
	subs := plan["sub_questions"].([]any)

	for i, sq := range subs {
		sub, ok := sq.(map[string]any)
		if !ok {
			t.Errorf("sub_question[%d] is not a map", i)
			continue
		}
		if sub["id"] == nil {
			t.Errorf("sub_question[%d] missing id", i)
		}
		if sub["question"] == nil || sub["question"] == "" {
			t.Errorf("sub_question[%d] missing question", i)
		}
		tools, ok := sub["suggested_tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Errorf("sub_question[%d] missing or empty suggested_tools", i)
		}
	}
}

func TestDecomposeQuery_SubQuestionsContainQuery(t *testing.T) {
	query := "renewable energy"
	subs := decomposeQuery(query, "standard")

	for _, sq := range subs {
		if !containsStr(sq.Question, query) {
			t.Errorf("sub_question %q does not contain the original query %q", sq.Question, query)
		}
	}
}

func TestBuildMarkdownReport_Structure(t *testing.T) {
	findings := []ResearchFinding{
		{
			SubQuestion: "What is X?",
			Content:     "X is a technology.",
			SourceURL:   "https://example.com",
			Confidence:  "high",
		},
		{
			SubQuestion: "What are the challenges?",
			Content:     "Challenge A and B.",
			SourceURL:   "https://test.org",
			Confidence:  "medium",
		},
	}

	report := buildMarkdownReport("test topic", "standard", []string{"https://provided.com"}, findings)

	// Check required sections exist
	requiredSections := []string{
		"# Research Report: test topic",
		"## Metadata",
		"## Abstract",
		"## Background",
		"## Findings",
		"## Analysis",
		"## Conclusion",
		"## Sources",
	}

	for _, section := range requiredSections {
		if !containsStr(report, section) {
			t.Errorf("report missing section: %s", section)
		}
	}

	// Check findings content
	if !containsStr(report, "X is a technology.") {
		t.Error("report missing finding 1 content")
	}
	if !containsStr(report, "Challenge A and B.") {
		t.Error("report missing finding 2 content")
	}

	// Check sources
	if !containsStr(report, "https://example.com") {
		t.Error("report missing source from finding")
	}
	if !containsStr(report, "https://test.org") {
		t.Error("report missing source from finding")
	}
	if !containsStr(report, "https://provided.com") {
		t.Error("report missing provided source")
	}

	// Check confidence info
	if !containsStr(report, "High confidence") {
		t.Error("report missing high confidence count in analysis")
	}
	if !containsStr(report, "Medium confidence") {
		t.Error("report missing medium confidence count in analysis")
	}
}

func TestBuildMarkdownReport_EmptyFindings(t *testing.T) {
	report := buildMarkdownReport("empty test", "quick", nil, nil)

	if !containsStr(report, "# Research Report: empty test") {
		t.Error("report missing title")
	}
	if !containsStr(report, "No external sources cited") {
		t.Error("report should indicate no sources when empty")
	}
}

func TestBuildAnalysisSummary_Distribution(t *testing.T) {
	findings := []ResearchFinding{
		{Content: "A", Confidence: "high"},
		{Content: "B", Confidence: "high"},
		{Content: "C", Confidence: "medium"},
		{Content: "D", Confidence: "low"},
	}

	summary := buildAnalysisSummary(findings)

	if !containsStr(summary, "Total findings: 4") {
		t.Error("analysis should show total findings count")
	}
	if !containsStr(summary, "High confidence: 2") {
		t.Error("analysis should show high confidence count")
	}
	if !containsStr(summary, "Medium confidence: 1") {
		t.Error("analysis should show medium confidence count")
	}
	if !containsStr(summary, "Low confidence: 1") {
		t.Error("analysis should show low confidence count")
	}
}

func TestBuildAnalysisSummary_SourceDiversity(t *testing.T) {
	findings := []ResearchFinding{
		{Content: "A", SourceURL: "https://a.com"},
		{Content: "B", SourceURL: "https://b.com"},
		{Content: "C", SourceURL: "https://a.com"}, // duplicate
	}

	summary := buildAnalysisSummary(findings)
	if !containsStr(summary, "2 unique source") {
		t.Error("analysis should count unique sources (2), got different count")
	}
}

func TestGroupFindingsByQuestion(t *testing.T) {
	findings := []ResearchFinding{
		{SubQuestion: "Q1", Content: "A"},
		{SubQuestion: "Q2", Content: "B"},
		{SubQuestion: "Q1", Content: "C"},
		{Content: "D"}, // no sub-question
	}

	grouped := groupFindingsByQuestion(findings)

	if len(grouped) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(grouped))
	}

	// Q1 group should have 2 findings
	if grouped[0].question != "Q1" || len(grouped[0].findings) != 2 {
		t.Errorf("group 0: question=%q, findings=%d, want Q1/2", grouped[0].question, len(grouped[0].findings))
	}
	// Q2 group should have 1 finding
	if grouped[1].question != "Q2" || len(grouped[1].findings) != 1 {
		t.Errorf("group 1: question=%q, findings=%d, want Q2/1", grouped[1].question, len(grouped[1].findings))
	}
	// General Findings group should have 1 finding
	if grouped[2].question != "General Findings" || len(grouped[2].findings) != 1 {
		t.Errorf("group 2: question=%q, findings=%d, want General Findings/1", grouped[2].question, len(grouped[2].findings))
	}
}

func TestBuildDocxContentBlocks_Structure(t *testing.T) {
	findings := []ResearchFinding{
		{SubQuestion: "What is X?", Content: "X is great.", SourceURL: "https://a.com", Confidence: "high"},
		{Content: "General finding.", Confidence: "medium"},
	}

	blocks := buildDocxContentBlocks("test topic", findings)

	// Should have at least: title heading, abstract heading, abstract paragraph,
	// findings heading, finding1 heading, finding1 paragraph, finding1 meta,
	// finding2 heading, finding2 paragraph, finding2 meta, sources heading, sources table
	if len(blocks) < 10 {
		t.Errorf("expected at least 10 content blocks, got %d", len(blocks))
	}

	// First block should be the title heading
	first, ok := blocks[0].(map[string]any)
	if !ok {
		t.Fatal("first block is not a map")
	}
	if first["type"] != "heading" {
		t.Errorf("first block type = %v, want heading", first["type"])
	}
	if !containsStr(first["text"].(string), "test topic") {
		t.Errorf("first block text = %v, should contain 'test topic'", first["text"])
	}
}

func TestBuildDocxContentBlocks_WithSources(t *testing.T) {
	findings := []ResearchFinding{
		{Content: "A", SourceURL: "https://example.com"},
		{Content: "B", SourceURL: "https://test.org"},
		{Content: "C"}, // no source
	}

	blocks := buildDocxContentBlocks("query", findings)

	// Find the sources table block
	var foundTable bool
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if block["type"] == "table" {
			foundTable = true
			headers, ok := block["headers"].([]any)
			if !ok || len(headers) == 0 {
				t.Error("table block missing headers")
			}
			rows, ok := block["rows"].([]any)
			if !ok || len(rows) != 2 {
				t.Errorf("table should have 2 rows (unique sources), got %v", len(rows))
			}
		}
	}
	if !foundTable {
		t.Error("expected a sources table block but none found")
	}
}

func TestHandleDeepResearch_PlanMode(t *testing.T) {
	args := map[string]any{
		"query": "artificial intelligence",
		"depth": "quick",
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["status"] != "plan_generated" {
		t.Errorf("status = %v, want plan_generated", parsed["status"])
	}
}

func TestHandleDeepResearch_EmptyQuery(t *testing.T) {
	args := map[string]any{
		"query": "",
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["error"] == nil {
		t.Error("expected error for empty query")
	}
}

func TestHandleDeepResearch_WhitespaceQuery(t *testing.T) {
	args := map[string]any{
		"query": "   ",
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["error"] == nil {
		t.Error("expected error for whitespace-only query")
	}
}

func TestHandleDeepResearch_CompileMode_InlineMarkdown(t *testing.T) {
	// No ObjectStore: should return inline markdown
	args := map[string]any{
		"query": "test topic",
		"findings": []any{
			map[string]any{
				"content":     "Test finding content",
				"source_url":  "https://example.com",
				"confidence":  "high",
				"sub_question": "What is it?",
			},
		},
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["status"] != "completed" {
		t.Errorf("status = %v, want completed", parsed["status"])
	}
	if parsed["format"] != "markdown" {
		t.Errorf("format = %v, want markdown", parsed["format"])
	}
	report, ok := parsed["report"].(string)
	if !ok || report == "" {
		t.Error("expected non-empty report field")
	}
	if report != "" {
		if !containsStr(report, "Research Report") {
			t.Error("report should contain 'Research Report'")
		}
		if !containsStr(report, "Test finding content") {
			t.Error("report should contain finding content")
		}
	}
}

func TestHandleDeepResearch_CompileMode_InvalidFindings(t *testing.T) {
	args := map[string]any{
		"query": "test",
		"findings": []any{
			map[string]any{}, // no content
		},
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["error"] == nil {
		t.Error("expected error for findings without content")
	}
}

func TestHandleDeepResearch_CompileMode_ExceedsMaxFindings(t *testing.T) {
	findings := make([]any, MaxFindings+1)
	for i := range findings {
		findings[i] = map[string]any{"content": "finding"}
	}

	args := map[string]any{
		"query":    "test",
		"findings": findings,
	}

	result := handleDeepResearch(context.Background(), args, nil)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if parsed["error"] == nil {
		t.Error("expected error for exceeding max findings")
	}
}

func TestBuildInstructions(t *testing.T) {
	instructions := buildInstructions("standard", "markdown", []string{"https://a.com"})

	if !containsStr(instructions, "web_search") {
		t.Error("instructions should mention web_search")
	}
	if !containsStr(instructions, "web_extract") {
		t.Error("instructions should mention web_extract")
	}
	if !containsStr(instructions, "https://a.com") {
		t.Error("instructions should include provided source")
	}
	if !containsStr(instructions, "markdown") {
		t.Error("instructions should mention output format")
	}
}

func TestBuildInstructions_NoSources(t *testing.T) {
	instructions := buildInstructions("quick", "docx", nil)

	if !containsStr(instructions, "docx") {
		t.Error("instructions should mention docx format")
	}
	// Should not contain the "provided sources" section
	if containsStr(instructions, "user has provided specific sources") {
		t.Error("instructions should not mention provided sources when none given")
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	// Test with a variable that's likely not set
	val := getEnvOrDefault("HERMESX_RESEARCH_TEST_VAR_NOT_SET_12345", "default")
	if val != "default" {
		t.Errorf("getEnvOrDefault() = %q, want %q", val, "default")
	}

	// Test with PATH which should be set on all systems
	val = getEnvOrDefault("PATH", "default")
	if val == "default" {
		t.Error("getEnvOrDefault('PATH') should not return default")
	}
}

func TestConstants(t *testing.T) {
	if MaxSubQuestions < 1 {
		t.Error("MaxSubQuestions should be positive")
	}
	if MaxFindings < 1 {
		t.Error("MaxFindings should be positive")
	}
	if DefaultResearchDepth != "standard" {
		t.Errorf("DefaultResearchDepth = %q, want %q", DefaultResearchDepth, "standard")
	}
	if DefaultOutputFormat != "markdown" {
		t.Errorf("DefaultOutputFormat = %q, want %q", DefaultOutputFormat, "markdown")
	}
}

func TestValidDepthLevels(t *testing.T) {
	for _, level := range []string{"quick", "standard", "deep"} {
		if !validDepthLevels[level] {
			t.Errorf("validDepthLevels missing %q", level)
		}
	}
	if len(validDepthLevels) != 3 {
		t.Errorf("validDepthLevels has %d entries, expected 3", len(validDepthLevels))
	}
}

func TestValidOutputFormats(t *testing.T) {
	for _, format := range []string{"markdown", "docx"} {
		if !validOutputFormats[format] {
			t.Errorf("validOutputFormats missing %q", format)
		}
	}
	if len(validOutputFormats) != 2 {
		t.Errorf("validOutputFormats has %d entries, expected 2", len(validOutputFormats))
	}
}

// containsStr is a helper that checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
