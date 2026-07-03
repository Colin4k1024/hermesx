package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/gateway"
)

// NotionAdapter implements PlatformAdapter for Notion.
type NotionAdapter struct {
	BasePlatformAdapter
	apiKey     string
	baseURL    string
	client     *http.Client
	pollTicker *time.Ticker
	stopCh     chan struct{}
}

// NewNotionAdapter creates a new Notion adapter.
func NewNotionAdapter(apiKey string) *NotionAdapter {
	return &NotionAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(gateway.PlatformNotion),
		apiKey:              apiKey,
		baseURL:             "https://api.notion.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Connect establishes a connection to Notion API.
func (a *NotionAdapter) Connect(ctx context.Context) error {
	// Verify API key by listing users
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/users", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	a.setHeaders(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to notion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notion api error: %d %s", resp.StatusCode, string(body))
	}

	a.BasePlatformAdapter.connected = true
	slog.Info("Connected to Notion API")
	return nil
}

// Disconnect cleanly disconnects from Notion.
func (a *NotionAdapter) Disconnect() error {
	if a.pollTicker != nil {
		a.pollTicker.Stop()
	}
	close(a.stopCh)
	a.BasePlatformAdapter.connected = false
	return nil
}

// Send sends a text message to a Notion page.
func (a *NotionAdapter) Send(ctx context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	// In Notion, chatID is the page ID
	pageID := chatID

	// Append block to page
	block := map[string]any{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]any{
			"rich_text": []map[string]any{
				{
					"type": "text",
					"text": map[string]any{
						"content": text,
					},
				},
			},
		},
	}

	body := map[string]any{
		"children": []map[string]any{block},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/blocks/%s/children", a.baseURL, pageID)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	a.setHeaders(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send to notion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notion api error: %d %s", resp.StatusCode, string(respBody))
	}

	return &gateway.SendResult{
		Success:   true,
		MessageID: fmt.Sprintf("notion-%d", time.Now().UnixNano()),
	}, nil
}

// SendTyping sends a typing indicator (Notion doesn't support this natively).
func (a *NotionAdapter) SendTyping(ctx context.Context, chatID string) error {
	// Notion doesn't have a native typing indicator
	return nil
}

// SendImage sends an image file to Notion.
func (a *NotionAdapter) SendImage(ctx context.Context, chatID string, imagePath string, caption string, metadata map[string]string) (*gateway.SendResult, error) {
	// Notion requires images to be hosted externally or uploaded
	// For now, send as a link
	text := fmt.Sprintf("![Image](%s)", imagePath)
	if caption != "" {
		text = caption + "\n" + text
	}
	return a.Send(ctx, chatID, text, metadata)
}

// SendVoice sends a voice/audio message to Notion.
func (a *NotionAdapter) SendVoice(ctx context.Context, chatID string, audioPath string, metadata map[string]string) (*gateway.SendResult, error) {
	text := fmt.Sprintf("[Voice Message](%s)", audioPath)
	return a.Send(ctx, chatID, text, metadata)
}

// SendDocument sends a document/file to Notion.
func (a *NotionAdapter) SendDocument(ctx context.Context, chatID string, filePath string, metadata map[string]string) (*gateway.SendResult, error) {
	text := fmt.Sprintf("[Document](%s)", filePath)
	return a.Send(ctx, chatID, text, metadata)
}

// setHeaders sets the required headers for Notion API requests.
func (a *NotionAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")
}
