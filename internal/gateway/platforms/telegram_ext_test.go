package platforms

import (
	"testing"
)

func TestInlineButton(t *testing.T) {
	// Test inline button struct creation.
	btn := InlineButton{Text: "Click me", CallbackData: "action:1"}
	if btn.Text != "Click me" {
		t.Errorf("Text = %q, want 'Click me'", btn.Text)
	}
	if btn.CallbackData != "action:1" {
		t.Errorf("CallbackData = %q, want 'action:1'", btn.CallbackData)
	}
}

func TestInlineButtonURL(t *testing.T) {
	btn := InlineButton{Text: "Visit", URL: "https://example.com"}
	if btn.URL != "https://example.com" {
		t.Errorf("URL = %q, want 'https://example.com'", btn.URL)
	}
}

func TestMediaGroupItem(t *testing.T) {
	item := MediaGroupItem{Type: "photo", Path: "/tmp/test.jpg", Caption: "Test"}
	if item.Type != "photo" {
		t.Errorf("Type = %q, want 'photo'", item.Type)
	}
}

func TestReconnectConfig(t *testing.T) {
	cfg := defaultReconnectConfig
	if cfg.maxRetries != 10 {
		t.Errorf("maxRetries = %d, want 10", cfg.maxRetries)
	}
	if cfg.initialBackoff.Seconds() != 2 {
		t.Errorf("initialBackoff = %v, want 2s", cfg.initialBackoff)
	}
}

func TestSendWithInlineKeyboard_NotConnected(t *testing.T) {
	adapter := &TelegramAdapter{}
	_, err := adapter.SendWithInlineKeyboard(nil, "123", "test", nil)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestSendMediaGroup_NotConnected(t *testing.T) {
	adapter := &TelegramAdapter{}
	_, err := adapter.SendMediaGroup(nil, "123", []MediaGroupItem{
		{Type: "photo", Path: "/a.jpg"},
		{Type: "photo", Path: "/b.jpg"},
	})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestSendMediaGroup_InvalidCount(t *testing.T) {
	adapter := &TelegramAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter("telegram"),
	}
	adapter.connected = true
	// Can't actually send without a real bot, but we can test validation.
	_, err := adapter.SendMediaGroup(nil, "123", []MediaGroupItem{
		{Type: "photo", Path: "/a.jpg"},
	})
	if err == nil {
		t.Error("expected error for < 2 items")
	}
}

func TestAnswerCallbackQuery_NotConnected(t *testing.T) {
	adapter := &TelegramAdapter{}
	err := adapter.AnswerCallbackQuery("query-1", "ok", false)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestEditInlineKeyboard_NotConnected(t *testing.T) {
	adapter := &TelegramAdapter{}
	err := adapter.EditInlineKeyboard(nil, "123", 1, nil)
	if err == nil {
		t.Error("expected error when not connected")
	}
}
