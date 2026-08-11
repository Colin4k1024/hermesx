package platforms

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/gateway"
)

func TestNewBasePlatformAdapter_Defaults(t *testing.T) {
	adapter := NewBasePlatformAdapter(gateway.PlatformTelegram)
	if adapter.platform != gateway.PlatformTelegram {
		t.Fatalf("platform = %v, want %v", adapter.platform, gateway.PlatformTelegram)
	}
	if adapter.maxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", adapter.maxRetries)
	}
	if adapter.retryDelay.Seconds() != 1.0 {
		t.Fatalf("retryDelay = %v, want 1s", adapter.retryDelay)
	}
	if adapter.connected {
		t.Fatal("connected should be false by default")
	}
	if adapter.messageHandler != nil {
		t.Fatal("messageHandler should be nil by default")
	}
}

func TestBasePlatformAdapter_Platform(t *testing.T) {
	adapter := NewBasePlatformAdapter(gateway.PlatformDiscord)
	if adapter.Platform() != gateway.PlatformDiscord {
		t.Fatalf("Platform() = %v, want %v", adapter.Platform(), gateway.PlatformDiscord)
	}
}

func TestBasePlatformAdapter_IsConnected(t *testing.T) {
	adapter := NewBasePlatformAdapter(gateway.PlatformSlack)
	if adapter.IsConnected() {
		t.Fatal("IsConnected() should be false by default")
	}
	// Set connected to true and verify.
	adapter.connected = true
	if !adapter.IsConnected() {
		t.Fatal("IsConnected() should be true after setting connected=true")
	}
}

func TestBasePlatformAdapter_OnMessageAndEmit(t *testing.T) {
	adapter := NewBasePlatformAdapter(gateway.PlatformTelegram)

	var received *gateway.MessageEvent
	adapter.OnMessage(func(event *gateway.MessageEvent) {
		received = event
	})

	event := &gateway.MessageEvent{
		Text:        "hello",
		MessageType: gateway.MessageTypeText,
		Source: gateway.SessionSource{
			Platform: gateway.PlatformTelegram,
			ChatID:   "chat-123",
			UserID:   "user-1",
		},
	}
	adapter.EmitMessage(event)

	if received == nil {
		t.Fatal("handler was not called")
	}
	if received.Text != "hello" {
		t.Fatalf("received.Text = %q, want %q", received.Text, "hello")
	}
	if received.Source.ChatID != "chat-123" {
		t.Fatalf("received.Source.ChatID = %q, want %q", received.Source.ChatID, "chat-123")
	}
}

func TestBasePlatformAdapter_EmitMessage_NoHandler(t *testing.T) {
	adapter := NewBasePlatformAdapter(gateway.PlatformTelegram)
	// Should not panic when no handler is registered.
	event := &gateway.MessageEvent{Text: "test"}
	adapter.EmitMessage(event) // no-op, must not panic
}

func TestTruncateMessage_ShortText(t *testing.T) {
	text := "short message"
	got := TruncateMessage(text, 100)
	if got != text {
		t.Fatalf("TruncateMessage short text: got %q, want %q", got, text)
	}
}

func TestTruncateMessage_ExactLength(t *testing.T) {
	text := "exactly ten"
	got := TruncateMessage(text, len(text))
	if got != text {
		t.Fatalf("TruncateMessage exact length: got %q, want %q", got, text)
	}
}

func TestTruncateMessage_LongText(t *testing.T) {
	text := strings.Repeat("a", 200)
	got := TruncateMessage(text, 50)
	if len(got) > 50 {
		// The result should be bounded. It's truncated + suffix.
		// The implementation tries word boundary, fallback is text[:maxLen-20] + suffix.
		t.Fatalf("TruncateMessage result too long: len=%d", len(got))
	}
	if !strings.Contains(got, "...(truncated)") {
		t.Fatal("TruncateMessage should contain truncation suffix")
	}
}

func TestTruncateMessage_LongTextWithSpaces(t *testing.T) {
	// Build text with spaces to test word-boundary truncation.
	text := "word " + strings.Repeat("x ", 100)
	got := TruncateMessage(text, 50)
	if !strings.Contains(got, "...(truncated)") {
		t.Fatal("TruncateMessage should contain truncation suffix")
	}
}

func TestTruncateMessage_ZeroMaxLen(t *testing.T) {
	text := "some text"
	got := TruncateMessage(text, 0)
	// maxLen <= 0 should use MaxMessageLength (4096).
	if got != text {
		t.Fatalf("TruncateMessage with maxLen=0 should use default; got %q, want %q", got, text)
	}
}

func TestSplitMessage_ShortText(t *testing.T) {
	text := "short"
	parts := SplitMessage(text, 100)
	if len(parts) != 1 {
		t.Fatalf("SplitMessage short: got %d parts, want 1", len(parts))
	}
	if parts[0] != text {
		t.Fatalf("SplitMessage short: got %q, want %q", parts[0], text)
	}
}

func TestSplitMessage_LongText(t *testing.T) {
	// 100 characters split into chunks of 30.
	text := strings.Repeat("a", 100)
	parts := SplitMessage(text, 30)
	if len(parts) < 2 {
		t.Fatalf("SplitMessage long: got %d parts, want at least 2", len(parts))
	}
	// Reassemble and verify total length.
	reassembled := strings.Join(parts, "")
	if reassembled != text {
		t.Fatalf("SplitMessage: reassembled length=%d, want %d", len(reassembled), len(text))
	}
}

func TestSplitMessage_WithNewlines(t *testing.T) {
	// Build text with newlines to test newline-based splitting.
	text := strings.Repeat("line content\n", 10) // ~130 chars
	parts := SplitMessage(text, 50)
	if len(parts) < 2 {
		t.Fatalf("SplitMessage with newlines: got %d parts, want at least 2", len(parts))
	}
	reassembled := strings.Join(parts, "")
	if reassembled != text {
		t.Fatal("SplitMessage: reassembled does not match original")
	}
}

func TestSplitMessage_EmptyText(t *testing.T) {
	parts := SplitMessage("", 100)
	if len(parts) != 1 {
		t.Fatalf("SplitMessage empty: got %d parts, want 1", len(parts))
	}
	if parts[0] != "" {
		t.Fatalf("SplitMessage empty: got %q, want empty string", parts[0])
	}
}

func TestSplitMessage_ZeroMaxLen(t *testing.T) {
	text := "short text"
	parts := SplitMessage(text, 0)
	// maxLen <= 0 should use MaxMessageLength (4096), so short text stays as one part.
	if len(parts) != 1 {
		t.Fatalf("SplitMessage with maxLen=0: got %d parts, want 1", len(parts))
	}
}

func TestExtractMediaFromResponse_NoMedia(t *testing.T) {
	text := "This is a normal response.\nNo media here."
	media, cleaned := ExtractMediaFromResponse(text)
	if len(media) != 0 {
		t.Fatalf("expected no media, got %d", len(media))
	}
	if cleaned != text {
		t.Errorf("cleaned text changed unexpectedly: got %q", cleaned)
	}
}

func TestExtractMediaFromResponse_WithPNG(t *testing.T) {
	text := "Here is an image:\nMEDIA: /tmp/image.png\nEnd of message."
	media, cleaned := ExtractMediaFromResponse(text)
	if len(media) != 1 {
		t.Fatalf("expected 1 media file, got %d", len(media))
	}
	if media[0].Path != "/tmp/image.png" {
		t.Errorf("media path = %q, want /tmp/image.png", media[0].Path)
	}
	if media[0].IsVoice {
		t.Error("png should not be IsVoice")
	}
	if strings.Contains(cleaned, "MEDIA:") {
		t.Error("cleaned text should not contain MEDIA: tag")
	}
}

func TestExtractMediaFromResponse_VoiceOgg(t *testing.T) {
	text := "MEDIA: /tmp/voice.ogg"
	media, _ := ExtractMediaFromResponse(text)
	if len(media) != 1 || !media[0].IsVoice {
		t.Errorf("ogg should be IsVoice=true, got %+v", media)
	}
}

func TestExtractMediaFromResponse_VoiceOpus(t *testing.T) {
	text := "MEDIA: /tmp/voice.opus"
	media, _ := ExtractMediaFromResponse(text)
	if len(media) != 1 || !media[0].IsVoice {
		t.Errorf("opus should be IsVoice=true, got %+v", media)
	}
}

func TestExtractMediaFromResponse_MultipleMedia(t *testing.T) {
	text := "MEDIA: /a.png\ntext\nMEDIA: /b.ogg"
	media, _ := ExtractMediaFromResponse(text)
	if len(media) != 2 {
		t.Fatalf("expected 2 media, got %d", len(media))
	}
}

func TestExtractMediaFromResponse_EmptyPath(t *testing.T) {
	text := "MEDIA:   \nnormal"
	media, cleaned := ExtractMediaFromResponse(text)
	if len(media) != 0 {
		t.Fatalf("empty MEDIA: path should not add media, got %d", len(media))
	}
	if strings.Contains(cleaned, "MEDIA:") {
		t.Error("cleaned text should not contain MEDIA: line even with empty path")
	}
}

func TestRetryWithBackoff_SuccessFirst(t *testing.T) {
	ctx := context.Background()
	calls := 0
	err := RetryWithBackoff(ctx, 3, time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	calls := 0
	err := RetryWithBackoff(ctx, 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_AllFail(t *testing.T) {
	retryErr := errors.New("always fails")
	ctx := context.Background()
	calls := 0
	err := RetryWithBackoff(ctx, 2, time.Millisecond, func() error {
		calls++
		return retryErr
	})
	if err != retryErr {
		t.Errorf("expected retryErr, got %v", err)
	}
	if calls != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := RetryWithBackoff(ctx, 5, time.Millisecond, func() error {
		calls++
		if calls == 1 {
			cancel()
		}
		return errors.New("fail")
	})
	// Should return context error after cancel.
	if err == nil {
		t.Error("expected error when context cancelled")
	}
}
