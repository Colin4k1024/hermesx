package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	// MaxSpreadsheetRows limits the number of data rows per sheet.
	MaxSpreadsheetRows = 100000
	// MaxSpreadsheetFileSize limits upload size to 50MB.
	MaxSpreadsheetFileSize = 50 * 1024 * 1024
)

func init() {
	Register(&ToolEntry{
		Name:    "generate_spreadsheet",
		Toolset: "document_generation",
		Schema: map[string]any{
			"name":        "generate_spreadsheet",
			"description": "Generate an Excel (.xlsx) spreadsheet with one or more sheets. Each sheet can have custom headers and data rows. Returns an artifact URL for downloading the file.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Title / filename for the spreadsheet (without extension)",
					},
					"sheets": map[string]any{
						"type":        "array",
						"description": "Array of sheet definitions, each with name, headers, and data rows",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{
									"type":        "string",
									"description": "Sheet name",
								},
								"headers": map[string]any{
									"type":        "array",
									"description": "Column header names",
									"items": map[string]any{
										"type": "string",
									},
								},
								"data": map[string]any{
									"type":        "array",
									"description": "Data rows, where each row is an array of cell values",
									"items": map[string]any{
										"type":  "array",
										"items": map[string]any{},
									},
								},
							},
							"required": []string{"name"},
						},
					},
				},
				"required": []string{"title", "sheets"},
			},
		},
		Handler: handleGenerateSpreadsheet,
		Emoji:   "\U0001f4ca",
	})
}

// handleGenerateSpreadsheet creates an Excel file using excelize (Go in-process)
// and uploads it to ObjectStore.
func handleGenerateSpreadsheet(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	title, _ := args["title"].(string)
	if title == "" {
		return `{"error":"title is required"}`
	}

	sheetsRaw, ok := args["sheets"].([]any)
	if !ok || len(sheetsRaw) == 0 {
		return `{"error":"sheets is required and must be a non-empty array"}`
	}

	// Validate and cap row count
	totalRows := 0
	for _, s := range sheetsRaw {
		sheet, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if data, ok := sheet["data"].([]any); ok {
			totalRows += len(data)
		}
	}
	if totalRows > MaxSpreadsheetRows {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Total data rows (%d) exceeds limit (%d). Reduce the amount of data.", totalRows, MaxSpreadsheetRows),
			"hint":  "Split large datasets into multiple spreadsheets or filter to a subset.",
		})
	}

	// Create the Excel file
	f := excelize.NewFile()
	defer f.Close()

	// Track whether we need to delete the default "Sheet1"
	defaultSheetUsed := false

	for i, s := range sheetsRaw {
		sheet, ok := s.(map[string]any)
		if !ok {
			return toJSON(map[string]any{"error": fmt.Sprintf("sheets[%d] must be an object", i)})
		}

		sheetName, _ := sheet["name"].(string)
		if sheetName == "" {
			sheetName = fmt.Sprintf("Sheet%d", i+1)
		}

		// Use the default sheet for the first entry, create new sheets for subsequent ones
		var sheetIndex int
		if i == 0 {
			// Rename the default "Sheet1"
			if err := f.SetSheetName("Sheet1", sheetName); err != nil {
				return toJSON(map[string]any{"error": fmt.Sprintf("Failed to rename default sheet: %v", err)})
			}
			sheetIndex, _ = f.GetSheetIndex(sheetName)
			defaultSheetUsed = true
		} else {
			sheetIndex, _ = f.NewSheet(sheetName)
		}

		// Write headers
		if headers, ok := sheet["headers"].([]any); ok && len(headers) > 0 {
			for colIdx, h := range headers {
				cell, err := excelize.CoordinatesToCellName(colIdx+1, 1)
				if err != nil {
					return toJSON(map[string]any{"error": fmt.Sprintf("Cell coordinate error: %v", err)})
				}
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", h))
			}
			// Style header row: bold
			style, _ := f.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true},
			})
			if len(headers) > 0 {
				endCell, _ := excelize.CoordinatesToCellName(len(headers), 1)
				f.SetCellStyle(sheetName, "A1", endCell, style)
			}
		}

		// Write data rows
		if data, ok := sheet["data"].([]any); ok {
			headerCount := 0
			if headers, ok := sheet["headers"].([]any); ok {
				headerCount = len(headers)
			}
			for rowIdx, row := range data {
				rowArr, ok := row.([]any)
				if !ok {
					continue
				}
				for colIdx, val := range rowArr {
					cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2) // +2: 1-based + header row
					if err != nil {
						continue
					}
					f.SetCellValue(sheetName, cell, val)
				}
				// If data has more columns than headers, auto-fit won't happen but data is still written
				_ = headerCount
			}
		}

		f.SetActiveSheet(sheetIndex)
	}

	// Remove unused default sheet if it was replaced
	if !defaultSheetUsed {
		idx, _ := f.GetSheetIndex("Sheet1")
		if idx >= 0 {
			f.DeleteSheet("Sheet1")
		}
	}

	// Write to temporary file
	tmpDir, err := os.MkdirTemp("", "hermesx-xlsx-")
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to create temp directory: %v", err)})
	}
	defer os.RemoveAll(tmpDir)

	// Sanitize title for filename
	safeTitle := sanitizeFilename(title)
	tmpPath := filepath.Join(tmpDir, safeTitle+".xlsx")

	if err := f.SaveAs(tmpPath); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to save spreadsheet: %v", err)})
	}

	// Read the file bytes
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to read generated file: %v", err)})
	}

	if len(data) > MaxSpreadsheetFileSize {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Generated file size (%d bytes) exceeds limit (%d bytes).", len(data), MaxSpreadsheetFileSize),
			"hint":  "Reduce the number of rows or columns.",
		})
	}

	// Upload to ObjectStore
	if tctx == nil || tctx.ObjectStore == nil {
		return toJSON(map[string]any{"error": "ObjectStore is not configured. File generation requires SaaS mode."})
	}

	key := buildArtifactKey(tctx.TenantID, tctx.TaskID, safeTitle+".xlsx")
	if err := tctx.ObjectStore.PutObject(ctx, key, data); err != nil {
		slog.Error("spreadsheet: failed to upload to object store", "error", err, "key", key)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Failed to upload spreadsheet: %v", err),
			"hint":  "The file was generated successfully but upload failed. Please retry.",
		})
	}

	return toJSON(map[string]any{
		"success":      true,
		"artifact_key": key,
		"filename":     safeTitle + ".xlsx",
		"size_bytes":   len(data),
		"sheets":       len(sheetsRaw),
		"total_rows":   totalRows,
		"message":      fmt.Sprintf("Spreadsheet '%s' generated successfully with %d sheet(s).", title, len(sheetsRaw)),
	})
}

// buildArtifactKey constructs the object store key for an artifact.
func buildArtifactKey(tenantID, taskID, filename string) string {
	if tenantID == "" {
		tenantID = "default"
	}
	if taskID == "" {
		taskID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("artifacts/%s/%s/%s", tenantID, taskID, filename)
}

// sanitizeFilename removes or replaces characters unsafe for filenames.
func sanitizeFilename(name string) string {
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", ".."}
	result := name
	for _, c := range unsafe {
		result = strings.ReplaceAll(result, c, "_")
	}
	result = strings.TrimSpace(result)
	if result == "" {
		result = "document"
	}
	// Cap length
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}
