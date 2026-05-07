package gateway

import (
	"context"
	"testing"
)

type mediaTestAdapter struct {
	platform   Platform
	sentImages []string
	sentVoice  []string
	sentDocs   []string
	sentTexts  []string
	failSend   bool
}

func (m *mediaTestAdapter) Platform() Platform                           { return m.platform }
func (m *mediaTestAdapter) Connect(_ context.Context) error              { return nil }
func (m *mediaTestAdapter) Disconnect() error                            { return nil }
func (m *mediaTestAdapter) SendTyping(_ context.Context, _ string) error { return nil }
func (m *mediaTestAdapter) OnMessage(_ func(event *MessageEvent))        {}
func (m *mediaTestAdapter) IsConnected() bool                            { return true }

func (m *mediaTestAdapter) Send(_ context.Context, _ string, text string, _ map[string]string) (*SendResult, error) {
	m.sentTexts = append(m.sentTexts, text)
	if m.failSend {
		return &SendResult{Success: false, Error: "failed"}, nil
	}
	return &SendResult{Success: true, MessageID: "txt-1"}, nil
}

func (m *mediaTestAdapter) SendImage(_ context.Context, _ string, path string, _ string, _ map[string]string) (*SendResult, error) {
	m.sentImages = append(m.sentImages, path)
	return &SendResult{Success: true, MessageID: "img-1"}, nil
}

func (m *mediaTestAdapter) SendVoice(_ context.Context, _ string, path string, _ map[string]string) (*SendResult, error) {
	m.sentVoice = append(m.sentVoice, path)
	return &SendResult{Success: true, MessageID: "voice-1"}, nil
}

func (m *mediaTestAdapter) SendDocument(_ context.Context, _ string, path string, _ map[string]string) (*SendResult, error) {
	m.sentDocs = append(m.sentDocs, path)
	return &SendResult{Success: true, MessageID: "doc-1"}, nil
}

func setupDispatcher(caps PlatformCapabilities) (*MediaDispatcher, *mediaTestAdapter) {
	reg := &Registry{registrations: make(map[Platform]*PlatformRegistration)}
	reg.Register(&PlatformRegistration{
		Platform:     PlatformTelegram,
		DisplayName:  "Test",
		Capabilities: caps,
	})
	adapter := &mediaTestAdapter{platform: PlatformTelegram}
	return NewMediaDispatcher(reg), adapter
}

func TestDispatch_ImageDirectly(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsImages: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		Path:     "/tmp/photo.jpg",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentImages) != 1 {
		t.Fatalf("expected 1 image sent, got %d", len(adapter.sentImages))
	}
	if result.FallbackUsed != "" {
		t.Errorf("expected no fallback, got %s", result.FallbackUsed)
	}
}

func TestDispatch_ImageFallbackToDocument(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsImages: false, SupportsDocuments: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		Path:     "/tmp/photo.jpg",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentDocs) != 1 {
		t.Fatalf("expected 1 document sent, got %d", len(adapter.sentDocs))
	}
	if result.FallbackUsed != MediaTypeDocument {
		t.Errorf("expected document fallback, got %s", result.FallbackUsed)
	}
}

func TestDispatch_ImageFallbackToLink(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		URL:      "https://example.com/img.png",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentTexts) != 1 {
		t.Fatalf("expected 1 text sent, got %d", len(adapter.sentTexts))
	}
	if result.FallbackUsed != "link" {
		t.Errorf("expected link fallback, got %s", result.FallbackUsed)
	}
}

func TestDispatch_VideoDirectly(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsVideo: true, SupportsDocuments: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeVideo,
		Path:     "/tmp/clip.mp4",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentDocs) != 1 {
		t.Fatalf("expected 1 document sent (video via document), got %d", len(adapter.sentDocs))
	}
	if result.FallbackUsed != "" {
		t.Errorf("expected no fallback for video-capable platform, got %s", result.FallbackUsed)
	}
}

func TestDispatch_VideoFallbackToDocument(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsVideo: false, SupportsDocuments: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeVideo,
		Path:     "/tmp/clip.mp4",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.FallbackUsed != MediaTypeDocument {
		t.Errorf("expected document fallback, got %s", result.FallbackUsed)
	}
}

func TestDispatch_VoiceDirectly(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsVoice: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeVoice,
		Path:     "/tmp/voice.ogg",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentVoice) != 1 {
		t.Fatalf("expected 1 voice sent, got %d", len(adapter.sentVoice))
	}
}

func TestDispatch_VoiceFallbackToDocument(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsVoice: false, SupportsDocuments: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeVoice,
		Path:     "/tmp/voice.ogg",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentDocs) != 1 {
		t.Fatal("expected document fallback for voice")
	}
	if result.FallbackUsed != MediaTypeDocument {
		t.Errorf("expected document fallback, got %s", result.FallbackUsed)
	}
}

func TestDispatch_DocumentDirectly(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsDocuments: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeDocument,
		Path:     "/tmp/report.pdf",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentDocs) != 1 {
		t.Fatal("expected 1 document sent")
	}
}

func TestDispatch_UnknownPlatformDefaultsCaps(t *testing.T) {
	reg := &Registry{registrations: make(map[Platform]*PlatformRegistration)}
	d := NewMediaDispatcher(reg)
	adapter := &mediaTestAdapter{platform: "unknown"}

	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		Path:     "/tmp/photo.jpg",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success with default caps")
	}
	if len(adapter.sentImages) != 1 {
		t.Fatal("default caps should allow images")
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		path     string
		expected MediaType
	}{
		{"/tmp/photo.jpg", MediaTypeImage},
		{"/tmp/photo.PNG", MediaTypeImage},
		{"/tmp/clip.mp4", MediaTypeVideo},
		{"/tmp/clip.webm", MediaTypeVideo},
		{"/tmp/voice.ogg", MediaTypeAudio},
		{"/tmp/song.mp3", MediaTypeAudio},
		{"/tmp/report.pdf", MediaTypeDocument},
		{"/tmp/data.csv", MediaTypeDocument},
		{"/tmp/unknown.xyz", MediaTypeDocument},
		{"https://example.com/file.gif", MediaTypeImage},
	}
	for _, tc := range tests {
		got := DetectMediaType(tc.path)
		if got != tc.expected {
			t.Errorf("DetectMediaType(%q) = %s, want %s", tc.path, got, tc.expected)
		}
	}
}

func TestDispatch_UsesURLWhenPathEmpty(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{SupportsImages: true})
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		URL:      "https://example.com/img.png",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(adapter.sentImages) != 1 || adapter.sentImages[0] != "https://example.com/img.png" {
		t.Error("should use URL when Path is empty")
	}
}

func TestDispatch_CaptionInLinkFallback(t *testing.T) {
	d, adapter := setupDispatcher(PlatformCapabilities{})
	d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeDocument,
		URL:      "https://example.com/file.pdf",
		Caption:  "Important doc",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if len(adapter.sentTexts) != 1 {
		t.Fatal("expected text fallback")
	}
	if adapter.sentTexts[0] == "" {
		t.Error("expected non-empty text with caption")
	}
}

func TestDispatch_MissingChatID(t *testing.T) {
	d, _ := setupDispatcher(PlatformCapabilities{SupportsImages: true})
	adapter := &mediaTestAdapter{platform: PlatformTelegram}
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type: MediaTypeImage,
		Path: "/tmp/photo.jpg",
	})
	if result.Success {
		t.Fatal("expected failure when chat_id missing")
	}
	if result.Error != ErrMissingChatID {
		t.Errorf("expected ErrMissingChatID, got %v", result.Error)
	}
}

func TestDispatch_PathTraversal(t *testing.T) {
	d, _ := setupDispatcher(PlatformCapabilities{SupportsImages: true})
	adapter := &mediaTestAdapter{platform: PlatformTelegram}
	result := d.Dispatch(context.Background(), adapter, MediaPayload{
		Type:     MediaTypeImage,
		Path:     "/tmp/../etc/passwd",
		Metadata: map[string]string{"chat_id": "123"},
	})
	if result.Success {
		t.Fatal("expected failure for path traversal")
	}
	if result.Error != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", result.Error)
	}
}
