package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

const (
	// MaxDocumentPages limits the number of content blocks.
	MaxDocumentPages = 200
)

func init() {
	Register(&ToolEntry{
		Name:    "generate_document",
		Toolset: "document_generation",
		Schema: map[string]any{
			"name":        "generate_document",
			"description": "Generate a Word document (.docx) with structured content blocks (headings, paragraphs, tables, images). Returns an artifact URL for downloading the file.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Document title (used as filename without extension)",
					},
					"content": map[string]any{
						"type":        "array",
						"description": "Array of content blocks. Each block has a 'type' and corresponding data.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"type": map[string]any{
									"type":        "string",
									"description": "Block type: heading, paragraph, table, or image",
									"enum":        []string{"heading", "paragraph", "table", "image"},
								},
								"text": map[string]any{
									"type":        "string",
									"description": "Text content for heading/paragraph blocks",
								},
								"level": map[string]any{
									"type":        "integer",
									"description": "Heading level (1-6), only for heading blocks",
								},
								"headers": map[string]any{
									"type":        "array",
									"description": "Column headers for table blocks",
									"items": map[string]any{
										"type": "string",
									},
								},
								"rows": map[string]any{
									"type":        "array",
									"description": "Data rows for table blocks, each row is an array of cell values",
									"items": map[string]any{
										"type":  "array",
										"items": map[string]any{},
									},
								},
								"url": map[string]any{
									"type":        "string",
									"description": "Image URL for image blocks",
								},
								"width": map[string]any{
									"type":        "number",
									"description": "Image width in inches (default: 6.0)",
								},
							},
							"required": []string{"type"},
						},
					},
				},
				"required": []string{"title", "content"},
			},
		},
		Handler: handleGenerateDocument,
		Emoji:   "\U0001f4c4",
	})
}

// handleGenerateDocument creates a .docx file via Python sandbox with python-docx.
func handleGenerateDocument(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	title, _ := args["title"].(string)
	if title == "" {
		return `{"error":"title is required"}`
	}

	contentRaw, ok := args["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		return `{"error":"content is required and must be a non-empty array of blocks"}`
	}

	if len(contentRaw) > MaxDocumentPages {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Content blocks (%d) exceed limit (%d). Reduce the document size.", len(contentRaw), MaxDocumentPages),
			"hint":  "Split large documents into multiple files.",
		})
	}

	// Build the Python script
	script, err := buildDocumentScript(title, contentRaw)
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	// Execute in sandbox
	sandboxMode := os.Getenv("SANDBOX_MODE")
	if sandboxMode == "" {
		return toJSON(map[string]any{
			"error": "SANDBOX_MODE is required for generate_document",
			"hint":  "Set SANDBOX_MODE=docker or SANDBOX_MODE=k8s-job for sandboxed execution.",
		})
	}

	safeTitle := sanitizeFilename(title)
	outputPath := DocGenOutputDir + "/" + safeTitle + ".docx"

	// Marshal parameters to JSON for stdin instead of embedding in script source.
	// This prevents script injection via user-supplied text content.
	stdinParams := map[string]any{
		"title":       title,
		"content":     contentRaw,
		"output_path": outputPath,
	}
	stdinData, err := json.Marshal(stdinParams)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to marshal params: %v", err)})
	}

	fileData, err := executePythonForFile(ctx, script, outputPath, sandboxMode, stdinData)
	if err != nil {
		slog.Error("document: sandbox execution failed", "error", err)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Document generation failed: %v", err),
			"hint":  "Check that python-docx is available in the sandbox image.",
		})
	}

	if len(fileData) > MaxDocGenFileSize {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Generated file size (%d bytes) exceeds limit (%d bytes).", len(fileData), MaxDocGenFileSize),
			"hint":  "Reduce the amount of content or image sizes.",
		})
	}

	// Upload to ObjectStore
	if tctx == nil || tctx.ObjectStore == nil {
		return toJSON(map[string]any{"error": "ObjectStore is not configured. File generation requires SaaS mode."})
	}

	key := buildArtifactKey(tctx.TenantID, tctx.TaskID, safeTitle+".docx")
	if err := tctx.ObjectStore.PutObject(ctx, key, fileData); err != nil {
		slog.Error("document: failed to upload to object store", "error", err, "key", key)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Failed to upload document: %v", err),
			"hint":  "The file was generated successfully but upload failed. Please retry.",
		})
	}

	return toJSON(map[string]any{
		"success":      true,
		"artifact_key": key,
		"filename":     safeTitle + ".docx",
		"size_bytes":   len(fileData),
		"blocks":       len(contentRaw),
		"message":      fmt.Sprintf("Document '%s' generated successfully with %d content block(s).", title, len(contentRaw)),
	})
}

// buildDocumentScript generates a Python script for creating a .docx document.
// The script reads its parameters from stdin as JSON to avoid embedding user
// text directly into Python source code (which would be a script injection risk).
func buildDocumentScript(title string, content []any) (string, error) {
	// Validate content blocks before generating script
	for i, block := range content {
		blockMap, ok := block.(map[string]any)
		if !ok {
			return "", fmt.Errorf("content[%d] must be an object", i)
		}
		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "heading", "paragraph", "table", "image":
			// valid
		default:
			return "", fmt.Errorf("content[%d]: unknown block type %q (valid: heading, paragraph, table, image)", i, blockType)
		}
	}

	return `from docx import Document
from docx.shared import Inches, Pt
from docx.enum.text import WD_ALIGN_PARAGRAPH
import os, sys, json, tempfile

params = json.loads(sys.stdin.read())
title = params["title"]
content = params["content"]
output_path = params["output_path"]

doc = Document()
doc.add_heading(title, level=0)

for i, block in enumerate(content):
    block_type = block["type"]
    if block_type == "heading":
        text = block.get("text", "")
        level = int(block.get("level", 1))
        if level < 1 or level > 6:
            level = 1
        doc.add_heading(text, level=level)
    elif block_type == "paragraph":
        text = block.get("text", "")
        doc.add_paragraph(text)
    elif block_type == "table":
        headers = block.get("headers", [])
        rows = block.get("rows", [])
        if len(headers) > 0:
            table = doc.add_table(rows=1, cols=len(headers), style='Table Grid')
            hdr_cells = table.rows[0].cells
            for j, h in enumerate(headers):
                hdr_cells[j].text = str(h)
        elif len(rows) > 0:
            first_row = rows[0]
            if isinstance(first_row, list):
                table = doc.add_table(rows=0, cols=len(first_row), style='Table Grid')
        for row in rows:
            if isinstance(row, list):
                row_cells = table.add_row().cells
                for j, val in enumerate(row):
                    row_cells[j].text = str(val)
    elif block_type == "image":
        url = block.get("url", "")
        if not url:
            continue
        width = block.get("width", 6.0)
        try:
            import urllib.request
            _img_path = os.path.join(tempfile.gettempdir(), 'img_' + str(i))
            urllib.request.urlretrieve(url, _img_path)
            doc.add_picture(_img_path, width=Inches(float(width)))
            os.remove(_img_path)
        except Exception as e:
            doc.add_paragraph("[Image load failed: " + str(e) + "]")

doc.save(output_path)
`, nil
}
