package platforms

import (
	"net/http"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/gateway"
)

func TestNewNotionAdapter(t *testing.T) {
	apiKey := "test-api-key"
	adapter := NewNotionAdapter(apiKey)

	if adapter == nil {
		t.Fatal("Expected adapter to be created")
	}

	if adapter.apiKey != apiKey {
		t.Errorf("Expected apiKey to be %s, got %s", apiKey, adapter.apiKey)
	}

	if adapter.baseURL != "https://api.notion.com/v1" {
		t.Errorf("Expected baseURL to be https://api.notion.com/v1, got %s", adapter.baseURL)
	}
}

func TestNotionAdapter_Platform(t *testing.T) {
	adapter := NewNotionAdapter("test-key")

	if adapter.Platform() != gateway.PlatformNotion {
		t.Errorf("Expected platform to be %s, got %s", gateway.PlatformNotion, adapter.Platform())
	}
}

func TestNotionAdapter_IsConnected(t *testing.T) {
	adapter := NewNotionAdapter("test-key")

	// Initially not connected
	if adapter.IsConnected() {
		t.Error("Expected adapter to be initially disconnected")
	}
}

func TestNotionAdapter_SendTyping(t *testing.T) {
	adapter := NewNotionAdapter("test-key")

	// SendTyping should return nil (Notion doesn't support typing indicators)
	err := adapter.SendTyping(nil, "test-chat-id")
	if err != nil {
		t.Errorf("Expected SendTyping to return nil, got %v", err)
	}
}

func TestNotionAdapter_setHeaders(t *testing.T) {
	adapter := NewNotionAdapter("test-api-key")

	// Create a test request
	req := &http.Request{}
	req.Header = make(http.Header)

	adapter.setHeaders(req)

	// Check Authorization header
	auth := req.Header.Get("Authorization")
	if auth != "Bearer test-api-key" {
		t.Errorf("Expected Authorization header to be 'Bearer test-api-key', got '%s'", auth)
	}

	// Check Notion-Version header
	version := req.Header.Get("Notion-Version")
	if version != "2022-06-28" {
		t.Errorf("Expected Notion-Version header to be '2022-06-28', got '%s'", version)
	}

	// Check Content-Type header
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type header to be 'application/json', got '%s'", contentType)
	}
}
