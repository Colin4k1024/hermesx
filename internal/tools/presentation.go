package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	// MaxPresentationSlides limits the number of slides.
	MaxPresentationSlides = 100
)

func init() {
	Register(&ToolEntry{
		Name:    "generate_presentation",
		Toolset: "document_generation",
		Schema: map[string]any{
			"name":        "generate_presentation",
			"description": "Generate a PowerPoint presentation (.pptx) with slides containing titles, bullet points, and images. Returns an artifact URL for downloading the file.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Presentation title (used as filename without extension)",
					},
					"slides": map[string]any{
						"type":        "array",
						"description": "Array of slide definitions, each with layout, title, content, and optional image",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"layout": map[string]any{
									"type":        "string",
									"description": "Slide layout: title, content, two_content, blank",
									"enum":        []string{"title", "content", "two_content", "blank"},
									"default":     "content",
								},
								"title": map[string]any{
									"type":        "string",
									"description": "Slide title text",
								},
								"content": map[string]any{
									"type":        "array",
									"description": "Bullet point text items",
									"items": map[string]any{
										"type": "string",
									},
								},
								"subtitle": map[string]any{
									"type":        "string",
									"description": "Subtitle text (for title layout slides)",
								},
								"image_url": map[string]any{
									"type":        "string",
									"description": "URL of an image to embed in the slide",
								},
								"image_width": map[string]any{
									"type":        "number",
									"description": "Image width in inches (default: 5.0)",
								},
							},
						},
					},
				},
				"required": []string{"title", "slides"},
			},
		},
		Handler: handleGeneratePresentation,
		Emoji:   "\U0001f4d1",
	})
}

// handleGeneratePresentation creates a .pptx file via Python sandbox with python-pptx.
func handleGeneratePresentation(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	title, _ := args["title"].(string)
	if title == "" {
		return `{"error":"title is required"}`
	}

	slidesRaw, ok := args["slides"].([]any)
	if !ok || len(slidesRaw) == 0 {
		return `{"error":"slides is required and must be a non-empty array"}`
	}

	if len(slidesRaw) > MaxPresentationSlides {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Slide count (%d) exceeds limit (%d). Reduce the presentation size.", len(slidesRaw), MaxPresentationSlides),
			"hint":  "Split large presentations into multiple files.",
		})
	}

	// Build the Python script
	script, err := buildPresentationScript(title, slidesRaw)
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	// Execute in sandbox
	sandboxMode := os.Getenv("SANDBOX_MODE")
	if sandboxMode == "" {
		return toJSON(map[string]any{
			"error": "SANDBOX_MODE is required for generate_presentation",
			"hint":  "Set SANDBOX_MODE=docker or SANDBOX_MODE=k8s-job for sandboxed execution.",
		})
	}

	safeTitle := sanitizeFilename(title)
	outputPath := DocGenOutputDir + "/" + safeTitle + ".pptx"

	fileData, err := executePythonForFile(ctx, script, outputPath, sandboxMode, nil)
	if err != nil {
		slog.Error("presentation: sandbox execution failed", "error", err)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Presentation generation failed: %v", err),
			"hint":  "Check that python-pptx is available in the sandbox image.",
		})
	}

	if len(fileData) > MaxDocGenFileSize {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Generated file size (%d bytes) exceeds limit (%d bytes).", len(fileData), MaxDocGenFileSize),
			"hint":  "Reduce the number of slides or image sizes.",
		})
	}

	// Upload to ObjectStore
	if tctx == nil || tctx.ObjectStore == nil {
		return toJSON(map[string]any{"error": "ObjectStore is not configured. File generation requires SaaS mode."})
	}

	key := buildArtifactKey(tctx.TenantID, tctx.TaskID, safeTitle+".pptx")
	if err := tctx.ObjectStore.PutObject(ctx, key, fileData); err != nil {
		slog.Error("presentation: failed to upload to object store", "error", err, "key", key)
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Failed to upload presentation: %v", err),
			"hint":  "The file was generated successfully but upload failed. Please retry.",
		})
	}

	return toJSON(map[string]any{
		"success":      true,
		"artifact_key": key,
		"filename":     safeTitle + ".pptx",
		"size_bytes":   len(fileData),
		"slide_count":  len(slidesRaw),
		"message":      fmt.Sprintf("Presentation '%s' generated successfully with %d slide(s).", title, len(slidesRaw)),
	})
}

// buildPresentationScript generates a Python script for creating a .pptx presentation.
func buildPresentationScript(title string, slides []any) (string, error) {
	var b strings.Builder
	b.WriteString(`from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN
import os, tempfile

prs = Presentation()
prs.slide_width = Inches(13.333)
prs.slide_height = Inches(7.5)

def add_title_slide(prs, title_text, subtitle_text=""):
    layout = prs.slide_layouts[0]  # Title Slide
    slide = prs.slides.add_slide(layout)
    slide.shapes.title.text = title_text
    if subtitle_text and len(slide.placeholders) > 1:
        slide.placeholders[1].text = subtitle_text
    return slide

def add_content_slide(prs, title_text, bullet_items):
    layout = prs.slide_layouts[1]  # Title and Content
    slide = prs.slides.add_slide(layout)
    slide.shapes.title.text = title_text
    body = slide.placeholders[1]
    tf = body.text_frame
    tf.clear()
    for idx, item in enumerate(bullet_items):
        p = tf.paragraphs[0] if idx == 0 else tf.add_paragraph()
        p.text = item
        p.level = 0
    return slide

def add_blank_slide(prs):
    layout = prs.slide_layouts[6]  # Blank
    return prs.slides.add_slide(layout)

`)

	// Add title slide
	b.WriteString(fmt.Sprintf("add_title_slide(prs, %q)\n\n", title))

	for i, slide := range slides {
		slideMap, ok := slide.(map[string]any)
		if !ok {
			return "", fmt.Errorf("slides[%d] must be an object", i)
		}

		layout, _ := slideMap["layout"].(string)
		slideTitle, _ := slideMap["title"].(string)
		subtitle, _ := slideMap["subtitle"].(string)

		switch layout {
		case "title":
			b.WriteString(fmt.Sprintf("add_title_slide(prs, %q, %q)\n", slideTitle, subtitle))

		case "blank":
			b.WriteString("add_blank_slide(prs)\n")

		case "content", "two_content", "":
			// Default to content layout
			if layout == "" {
				layout = "content"
			}
			var contentItems []string
			if content, ok := slideMap["content"].([]any); ok {
				for _, item := range content {
					if s, ok := item.(string); ok {
						contentItems = append(contentItems, s)
					}
				}
			}
			b.WriteString(fmt.Sprintf("_slide_title = %q\n", slideTitle))
			b.WriteString("_slide_items = [")
			for j, item := range contentItems {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(fmt.Sprintf("%q", item))
			}
			b.WriteString("]\n")
			b.WriteString("add_content_slide(prs, _slide_title, _slide_items)\n")

		default:
			return "", fmt.Errorf("slides[%d]: unknown layout %q (valid: title, content, two_content, blank)", i, layout)
		}

		// Handle image if specified
		if imageURL, ok := slideMap["image_url"].(string); ok && imageURL != "" {
			imgWidth := 5.0
			if w, ok := slideMap["image_width"].(float64); ok && w > 0 {
				imgWidth = w
			}
			b.WriteString(fmt.Sprintf(`
try:
    import urllib.request
    _img_path = os.path.join(tempfile.gettempdir(), 'slide_img_%d')
    urllib.request.urlretrieve(%q, _img_path)
    _slide = prs.slides[-1]
    _slide.shapes.add_picture(_img_path, Inches(0.5), Inches(1.5), width=Inches(%.1f))
    os.remove(_img_path)
except Exception as e:
    _slide = prs.slides[-1]
    _txBox = _slide.shapes.add_textbox(Inches(1), Inches(3), Inches(8), Inches(1))
    _txBox.text_frame.text = "[Image load failed: " + str(e) + "]"
`, i, imageURL, imgWidth))
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("prs.save(%q)\n", DocGenOutputDir+"/"+sanitizeFilename(title)+".pptx"))

	return b.String(), nil
}
