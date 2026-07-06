package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	// DefaultResearchDepth is the default depth level for research.
	DefaultResearchDepth = "standard"
	// DefaultOutputFormat is the default output format for research reports.
	DefaultOutputFormat = "markdown"

	// MaxSubQuestions limits the number of sub-questions per research plan.
	MaxSubQuestions = 20

	// MaxFindings limits the number of findings in a compiled report.
	MaxFindings = 50
)

// validDepthLevels defines the accepted research depth values.
var validDepthLevels = map[string]bool{
	"quick":    true,
	"standard": true,
	"deep":     true,
}

// validOutputFormats defines the accepted output format values.
var validOutputFormats = map[string]bool{
	"markdown": true,
	"docx":     true,
}

// subQuestionCountByDepth controls how many sub-questions each depth generates.
var subQuestionCountByDepth = map[string]int{
	"quick":    3,
	"standard": 6,
	"deep":     10,
}

func init() {
	Register(&ToolEntry{
		Name:    "deep_research",
		Toolset: "research",
		Schema: map[string]any{
			"name":        "deep_research",
			"description": "Orchestrate multi-source deep research on a topic. Returns a structured research plan with sub-questions and suggested tools (web_search, web_extract, browse) for the Agent to execute. The Agent should fulfill each sub-question using the suggested tools, then call this tool again with findings to compile a final report.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The research topic or question to investigate",
					},
					"depth": map[string]any{
						"type":        "string",
						"description": "Research depth: 'quick' (3 sub-questions), 'standard' (6 sub-questions), 'deep' (10 sub-questions)",
						"enum":        []string{"quick", "standard", "deep"},
						"default":     "standard",
					},
					"sources": map[string]any{
						"type":        "array",
						"description": "Specific URLs to include as primary sources in the research",
						"items": map[string]any{
							"type": "string",
						},
					},
					"output_format": map[string]any{
						"type":        "string",
						"description": "Output format: 'markdown' (default) or 'docx' (Word document)",
						"enum":        []string{"markdown", "docx"},
						"default":     "markdown",
					},
					"findings": map[string]any{
						"type":        "array",
						"description": "Research findings collected by the Agent. Each finding has content, source_url, and confidence. When provided, the tool compiles findings into a final report instead of generating a plan.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"sub_question": map[string]any{
									"type":        "string",
									"description": "The sub-question this finding addresses",
								},
								"content": map[string]any{
									"type":        "string",
									"description": "The research content/findings",
								},
								"source_url": map[string]any{
									"type":        "string",
									"description": "Source URL where the finding was obtained",
								},
								"confidence": map[string]any{
									"type":        "string",
									"description": "Confidence level: 'high', 'medium', or 'low'",
									"enum":        []string{"high", "medium", "low"},
								},
							},
							"required": []string{"content"},
						},
					},
				},
				"required": []string{"query"},
			},
		},
		Handler: handleDeepResearch,
		Emoji:   "\U0001F50D",
	})
}

// ResearchPlan represents the structured research plan returned to the Agent.
type ResearchPlan struct {
	Query        string        `json:"query"`
	Depth        string        `json:"depth"`
	SubQuestions []SubQuestion `json:"sub_questions"`
	Sources      []string      `json:"sources,omitempty"`
	OutputFormat string        `json:"output_format"`
	Instructions string        `json:"instructions"`
}

// SubQuestion represents a single research sub-question with tool suggestions.
type SubQuestion struct {
	ID               int      `json:"id"`
	Question         string   `json:"question"`
	SuggestedTools   []string `json:"suggested_tools"`
	SuggestedQueries []string `json:"suggested_queries,omitempty"`
}

// ResearchFinding represents a single finding for report compilation.
type ResearchFinding struct {
	SubQuestion string `json:"sub_question,omitempty"`
	Content     string `json:"content"`
	SourceURL   string `json:"source_url,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
}

// handleDeepResearch is the main handler for the deep_research tool.
// It operates in two modes:
// 1. Plan mode (no findings): generates a research plan with sub-questions
// 2. Compile mode (with findings): compiles findings into a structured report
func handleDeepResearch(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	query, _ := args["query"].(string)
	if query = strings.TrimSpace(query); query == "" {
		return `{"error":"query is required"}`
	}

	// Parse optional parameters
	depth := parseDepth(args)
	outputFormat := parseOutputFormat(args)
	sources := parseSources(args)

	// Check if findings are provided (compile mode)
	findingsRaw, hasFindings := args["findings"].([]any)

	if !hasFindings || len(findingsRaw) == 0 {
		// Plan mode: generate research plan
		return generateResearchPlan(query, depth, sources, outputFormat)
	}

	// Compile mode: compile findings into report
	findings := parseFindings(findingsRaw)
	if len(findings) == 0 {
		return `{"error":"findings array must contain at least one valid finding with content"}`
	}
	if len(findings) > MaxFindings {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("findings count (%d) exceeds limit (%d)", len(findings), MaxFindings),
		})
	}

	return compileResearchReport(ctx, query, depth, sources, outputFormat, findings, tctx)
}

// parseDepth extracts and validates the depth parameter.
func parseDepth(args map[string]any) string {
	depth, _ := args["depth"].(string)
	depth = strings.ToLower(strings.TrimSpace(depth))
	if depth == "" || !validDepthLevels[depth] {
		return DefaultResearchDepth
	}
	return depth
}

// parseOutputFormat extracts and validates the output_format parameter.
func parseOutputFormat(args map[string]any) string {
	format, _ := args["output_format"].(string)
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || !validOutputFormats[format] {
		return DefaultOutputFormat
	}
	return format
}

// parseSources extracts the sources array from arguments.
func parseSources(args map[string]any) []string {
	sourcesRaw, ok := args["sources"].([]any)
	if !ok || len(sourcesRaw) == 0 {
		return nil
	}
	var sources []string
	for _, s := range sourcesRaw {
		if str, ok := s.(string); ok && strings.TrimSpace(str) != "" {
			sources = append(sources, strings.TrimSpace(str))
		}
	}
	return sources
}

// parseFindings converts raw findings array into typed structs.
func parseFindings(raw []any) []ResearchFinding {
	var findings []ResearchFinding
	for _, f := range raw {
		fm, ok := f.(map[string]any)
		if !ok {
			continue
		}
		content, _ := fm["content"].(string)
		if content = strings.TrimSpace(content); content == "" {
			continue
		}
		finding := ResearchFinding{
			Content: content,
		}
		if sq, ok := fm["sub_question"].(string); ok {
			finding.SubQuestion = strings.TrimSpace(sq)
		}
		if url, ok := fm["source_url"].(string); ok {
			finding.SourceURL = strings.TrimSpace(url)
		}
		if conf, ok := fm["confidence"].(string); ok {
			conf = strings.ToLower(strings.TrimSpace(conf))
			if conf == "high" || conf == "medium" || conf == "low" {
				finding.Confidence = conf
			}
		}
		findings = append(findings, finding)
	}
	return findings
}

// generateResearchPlan creates a structured research plan with sub-questions.
func generateResearchPlan(query, depth string, sources []string, outputFormat string) string {
	subQuestions := decomposeQuery(query, depth)

	plan := ResearchPlan{
		Query:        query,
		Depth:        depth,
		SubQuestions: subQuestions,
		Sources:      sources,
		OutputFormat: outputFormat,
		Instructions: buildInstructions(depth, outputFormat, sources),
	}

	return toJSON(map[string]any{
		"status":             "plan_generated",
		"plan":               plan,
		"next_step":          "Execute each sub-question using the suggested tools, then call deep_research again with the findings array to compile the final report.",
		"sub_question_count": len(subQuestions),
	})
}

// decomposeQuery breaks a research query into sub-questions based on depth.
// v1 uses a deterministic decomposition strategy; future versions can use LLM.
func decomposeQuery(query, depth string) []SubQuestion {
	count := subQuestionCountByDepth[depth]
	if count == 0 {
		count = subQuestionCountByDepth[DefaultResearchDepth]
	}

	// Deterministic sub-question templates covering common research dimensions
	templates := []struct {
		question       string
		tools          []string
		queryTemplates []string
	}{
		{
			question:       fmt.Sprintf("What is the current state and definition of: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{query, fmt.Sprintf("%s definition overview", query)},
		},
		{
			question:       fmt.Sprintf("What are the key developments and recent news about: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s latest developments 2024 2025", query), fmt.Sprintf("%s news", query)},
		},
		{
			question:       fmt.Sprintf("What are the main approaches, methods, or technologies related to: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s approaches methods", query), fmt.Sprintf("%s technology comparison", query)},
		},
		{
			question:       fmt.Sprintf("What are the challenges and limitations of: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s challenges problems limitations", query)},
		},
		{
			question:       fmt.Sprintf("What are expert opinions and analysis on: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s expert analysis opinion", query), fmt.Sprintf("%s research paper", query)},
		},
		{
			question:       fmt.Sprintf("What are practical applications and case studies of: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s case study application example", query)},
		},
		{
			question:       fmt.Sprintf("What is the historical context and evolution of: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s history evolution timeline", query)},
		},
		{
			question:       fmt.Sprintf("What are the future trends and predictions for: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s future trends predictions 2025 2026", query)},
		},
		{
			question:       fmt.Sprintf("What are the competing or alternative approaches to: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s alternatives comparison versus", query)},
		},
		{
			question:       fmt.Sprintf("What quantitative data and statistics exist about: %s?", query),
			tools:          []string{"web_search", "web_extract"},
			queryTemplates: []string{fmt.Sprintf("%s statistics data metrics numbers", query)},
		},
	}

	var subQuestions []SubQuestion
	for i := 0; i < count && i < len(templates); i++ {
		tmpl := templates[i]
		sq := SubQuestion{
			ID:               i + 1,
			Question:         tmpl.question,
			SuggestedTools:   tmpl.tools,
			SuggestedQueries: tmpl.queryTemplates,
		}
		subQuestions = append(subQuestions, sq)
	}

	return subQuestions
}

// buildInstructions generates the orchestration instructions for the Agent.
func buildInstructions(depth, outputFormat string, sources []string) string {
	var b strings.Builder

	b.WriteString("Research Orchestration Instructions:\n\n")
	b.WriteString("1. For each sub-question above, use the suggested tools to gather information.\n")
	b.WriteString("2. For 'web_search': search with the suggested queries, pick the most relevant results.\n")
	b.WriteString("3. For 'web_extract': extract detailed content from the most promising URLs.\n")

	if len(sources) > 0 {
		b.WriteString("4. The user has provided specific sources. Extract content from these URLs as primary sources:\n")
		for _, src := range sources {
			b.WriteString(fmt.Sprintf("   - %s\n", src))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("5. After gathering findings, call deep_research again with the same query and depth, adding a 'findings' array.\n"))
	b.WriteString(fmt.Sprintf("6. The final report will be generated in '%s' format.\n", outputFormat))

	if outputFormat == "docx" {
		b.WriteString("7. For docx output, ensure ObjectStore is configured (SaaS mode).\n")
	}

	return b.String()
}

// compileResearchReport compiles findings into a structured report.
func compileResearchReport(ctx context.Context, query, depth string, sources []string, outputFormat string, findings []ResearchFinding, tctx *ToolContext) string {
	report := buildMarkdownReport(query, depth, sources, findings)

	if outputFormat == "markdown" {
		return compileMarkdownOutput(ctx, query, report, tctx)
	}

	// docx output: generate .docx and upload
	return compileDocxOutput(ctx, query, findings, tctx)
}

// buildMarkdownReport generates a structured markdown report from findings.
func buildMarkdownReport(query, depth string, sources []string, findings []ResearchFinding) string {
	var b strings.Builder

	// Title
	b.WriteString(fmt.Sprintf("# Research Report: %s\n\n", query))

	// Metadata
	b.WriteString("## Metadata\n\n")
	b.WriteString(fmt.Sprintf("- **Query**: %s\n", query))
	b.WriteString(fmt.Sprintf("- **Depth**: %s\n", depth))
	b.WriteString(fmt.Sprintf("- **Generated**: %s\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))
	b.WriteString(fmt.Sprintf("- **Findings**: %d\n", len(findings)))
	if len(sources) > 0 {
		b.WriteString(fmt.Sprintf("- **Provided Sources**: %d\n", len(sources)))
	}
	b.WriteString("\n---\n\n")

	// Abstract / Summary
	b.WriteString("## Abstract\n\n")
	b.WriteString(fmt.Sprintf("This report presents research findings on the topic: **%s**. ", query))
	b.WriteString(fmt.Sprintf("A total of %d findings were collected across %s depth analysis.\n\n", len(findings), depth))

	// Group findings by sub-question
	grouped := groupFindingsByQuestion(findings)

	// Background section
	b.WriteString("## Background\n\n")
	if len(grouped) > 0 {
		for _, g := range grouped {
			if g.question != "" {
				b.WriteString(fmt.Sprintf("- %s\n", g.question))
			}
		}
	} else {
		b.WriteString("Research was conducted based on the provided query and available sources.\n")
	}
	b.WriteString("\n")

	// Findings section
	b.WriteString("## Findings\n\n")
	for i, finding := range findings {
		b.WriteString(fmt.Sprintf("### Finding %d", i+1))
		if finding.SubQuestion != "" {
			b.WriteString(fmt.Sprintf(": %s", finding.SubQuestion))
		}
		b.WriteString("\n\n")
		b.WriteString(finding.Content)
		b.WriteString("\n\n")

		if finding.SourceURL != "" {
			b.WriteString(fmt.Sprintf("**Source**: [%s](%s)\n", finding.SourceURL, finding.SourceURL))
		}
		if finding.Confidence != "" {
			b.WriteString(fmt.Sprintf("**Confidence**: %s\n", finding.Confidence))
		}
		b.WriteString("\n")
	}

	// Analysis section
	b.WriteString("## Analysis\n\n")
	b.WriteString(buildAnalysisSummary(findings))
	b.WriteString("\n")

	// Conclusion
	b.WriteString("## Conclusion\n\n")
	b.WriteString(fmt.Sprintf("Based on %d findings collected through %s-depth research on \"%s\", ", len(findings), depth, query))
	b.WriteString("the above findings provide a comprehensive overview of the topic. ")
	b.WriteString("Readers should consider the confidence levels and source diversity when drawing conclusions.\n\n")

	// Sources
	b.WriteString("## Sources\n\n")
	sourceSet := make(map[string]bool)
	for _, s := range sources {
		sourceSet[s] = true
	}
	for _, finding := range findings {
		if finding.SourceURL != "" {
			sourceSet[finding.SourceURL] = true
		}
	}
	if len(sourceSet) > 0 {
		idx := 1
		for src := range sourceSet {
			b.WriteString(fmt.Sprintf("%d. [%s](%s)\n", idx, src, src))
			idx++
		}
	} else {
		b.WriteString("No external sources cited.\n")
	}

	return b.String()
}

// groupedQuestion holds findings grouped by their sub-question.
type groupedQuestion struct {
	question string
	findings []ResearchFinding
}

// groupFindingsByQuestion groups findings by their sub_question field.
func groupFindingsByQuestion(findings []ResearchFinding) []groupedQuestion {
	order := make([]string, 0)
	groups := make(map[string][]ResearchFinding)

	for _, f := range findings {
		q := f.SubQuestion
		if q == "" {
			q = "General Findings"
		}
		if _, exists := groups[q]; !exists {
			order = append(order, q)
		}
		groups[q] = append(groups[q], f)
	}

	var result []groupedQuestion
	for _, q := range order {
		result = append(result, groupedQuestion{question: q, findings: groups[q]})
	}
	return result
}

// buildAnalysisSummary generates an analysis summary based on findings.
func buildAnalysisSummary(findings []ResearchFinding) string {
	var b strings.Builder

	// Confidence distribution
	high, medium, low := 0, 0, 0
	for _, f := range findings {
		switch f.Confidence {
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	total := len(findings)
	b.WriteString(fmt.Sprintf("**Finding Distribution**:\n"))
	b.WriteString(fmt.Sprintf("- Total findings: %d\n", total))
	if high > 0 {
		b.WriteString(fmt.Sprintf("- High confidence: %d (%.0f%%)\n", high, float64(high)/float64(total)*100))
	}
	if medium > 0 {
		b.WriteString(fmt.Sprintf("- Medium confidence: %d (%.0f%%)\n", medium, float64(medium)/float64(total)*100))
	}
	if low > 0 {
		b.WriteString(fmt.Sprintf("- Low confidence: %d (%.0f%%)\n", low, float64(low)/float64(total)*100))
	}

	// Source diversity
	sourceSet := make(map[string]bool)
	for _, f := range findings {
		if f.SourceURL != "" {
			sourceSet[f.SourceURL] = true
		}
	}
	if len(sourceSet) > 0 {
		b.WriteString(fmt.Sprintf("\n**Source Diversity**: %d unique source(s)\n", len(sourceSet)))
	}

	return b.String()
}

// compileMarkdownOutput handles markdown output: upload to ObjectStore and return result.
func compileMarkdownOutput(ctx context.Context, query, report string, tctx *ToolContext) string {
	if tctx == nil || tctx.ObjectStore == nil {
		// No ObjectStore: return report directly in the response
		return toJSON(map[string]any{
			"status":  "completed",
			"format":  "markdown",
			"report":  report,
			"message": "Report generated. ObjectStore not available; report returned inline.",
		})
	}

	safeTitle := sanitizeFilename(query)
	filename := fmt.Sprintf("research_%s.md", safeTitle)
	key := buildArtifactKey(tctx.TenantID, tctx.TaskID, filename)

	if err := tctx.ObjectStore.PutObject(ctx, key, []byte(report)); err != nil {
		slog.Error("research: failed to upload markdown report", "error", err, "key", key)
		// Fallback: return inline
		return toJSON(map[string]any{
			"status": "completed",
			"format": "markdown",
			"report": report,
			"error":  fmt.Sprintf("Upload failed: %v. Report returned inline.", err),
		})
	}

	return toJSON(map[string]any{
		"status":        "completed",
		"format":        "markdown",
		"artifact_key":  key,
		"filename":      filename,
		"report_length": len(report),
		"message":       fmt.Sprintf("Research report '%s' generated and uploaded successfully.", filename),
	})
}

// compileDocxOutput handles docx output by building content blocks and uploading.
func compileDocxOutput(ctx context.Context, query string, findings []ResearchFinding, tctx *ToolContext) string {
	if tctx == nil || tctx.ObjectStore == nil {
		return toJSON(map[string]any{
			"error": "ObjectStore is required for docx output. Use markdown format or configure SaaS mode.",
			"hint":  "Set output_format to 'markdown' to get inline results.",
		})
	}

	// Build content blocks for generate_document
	blocks := buildDocxContentBlocks(query, findings)

	// Generate docx via the sandbox
	sandboxMode := getEnvOrDefault("SANDBOX_MODE", "")
	if sandboxMode == "" {
		return toJSON(map[string]any{
			"error": "SANDBOX_MODE is required for docx generation",
			"hint":  "Set SANDBOX_MODE=docker or SANDBOX_MODE=k8s-job, or use output_format='markdown'.",
		})
	}

	safeTitle := sanitizeFilename(fmt.Sprintf("research_%s", query))
	outputPath := DocGenOutputDir + "/" + safeTitle + ".docx"

	script, err := buildDocumentScript(fmt.Sprintf("Research Report: %s", query), blocks)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to build document script: %v", err)})
	}

	// Marshal parameters to JSON for stdin (script reads from stdin instead of embedding data).
	stdinParams := map[string]any{
		"title":       fmt.Sprintf("Research Report: %s", query),
		"content":     blocks,
		"output_path": outputPath,
	}
	stdinData, err := json.Marshal(stdinParams)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to marshal params: %v", err)})
	}

	fileData, err := executePythonForFile(ctx, script, outputPath, sandboxMode, stdinData)
	if err != nil {
		slog.Error("research: docx generation failed", "error", err)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Document generation failed: %v", err),
			"hint":  "Ensure python-docx is available in the sandbox image.",
		})
	}

	if len(fileData) > MaxDocGenFileSize {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Generated file (%d bytes) exceeds limit (%d bytes).", len(fileData), MaxDocGenFileSize),
		})
	}

	key := buildArtifactKey(tctx.TenantID, tctx.TaskID, safeTitle+".docx")
	if err := tctx.ObjectStore.PutObject(ctx, key, fileData); err != nil {
		slog.Error("research: failed to upload docx", "error", err, "key", key)
		return toJSON(map[string]any{"error": fmt.Sprintf("Upload failed: %v", err)})
	}

	return toJSON(map[string]any{
		"status":       "completed",
		"format":       "docx",
		"artifact_key": key,
		"filename":     safeTitle + ".docx",
		"size_bytes":   len(fileData),
		"message":      fmt.Sprintf("Research report generated as .docx (%d findings).", len(findings)),
	})
}

// buildDocxContentBlocks converts findings into generate_document content blocks.
func buildDocxContentBlocks(query string, findings []ResearchFinding) []any {
	var blocks []any

	// Title heading
	blocks = append(blocks, map[string]any{
		"type": "heading", "text": fmt.Sprintf("Research Report: %s", query), "level": 1,
	})

	// Abstract
	blocks = append(blocks, map[string]any{
		"type": "heading", "text": "Abstract", "level": 2,
	})
	blocks = append(blocks, map[string]any{
		"type": "paragraph",
		"text": fmt.Sprintf("This report presents %d research findings on: %s", len(findings), query),
	})

	// Findings
	blocks = append(blocks, map[string]any{
		"type": "heading", "text": "Findings", "level": 2,
	})
	for i, f := range findings {
		heading := fmt.Sprintf("Finding %d", i+1)
		if f.SubQuestion != "" {
			heading = fmt.Sprintf("Finding %d: %s", i+1, f.SubQuestion)
		}
		blocks = append(blocks, map[string]any{
			"type": "heading", "text": heading, "level": 3,
		})
		blocks = append(blocks, map[string]any{
			"type": "paragraph", "text": f.Content,
		})
		if f.SourceURL != "" || f.Confidence != "" {
			detail := ""
			if f.SourceURL != "" {
				detail += "Source: " + f.SourceURL
			}
			if f.Confidence != "" {
				if detail != "" {
					detail += " | "
				}
				detail += "Confidence: " + f.Confidence
			}
			blocks = append(blocks, map[string]any{
				"type": "paragraph", "text": detail,
			})
		}
	}

	// Sources table
	sourceSet := make(map[string]bool)
	for _, f := range findings {
		if f.SourceURL != "" {
			sourceSet[f.SourceURL] = true
		}
	}
	if len(sourceSet) > 0 {
		blocks = append(blocks, map[string]any{
			"type": "heading", "text": "Sources", "level": 2,
		})
		rows := make([]any, 0, len(sourceSet))
		for src := range sourceSet {
			rows = append(rows, []any{src})
		}
		blocks = append(blocks, map[string]any{
			"type":    "table",
			"headers": []any{"Source URL"},
			"rows":    rows,
		})
	}

	return blocks
}

// getEnvOrDefault returns the environment variable value or the default.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
