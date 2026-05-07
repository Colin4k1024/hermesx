package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

// MediaType classifies a media file for dispatch routing.
type MediaType string

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeVoice    MediaType = "voice"
	MediaTypeDocument MediaType = "document"
)

// MediaPayload represents a media item to be sent through the gateway.
type MediaPayload struct {
	Type     MediaType
	Path     string
	URL      string
	Caption  string
	Filename string
	Metadata map[string]string
}

// MediaDispatchResult holds the outcome of a media dispatch attempt.
type MediaDispatchResult struct {
	Success      bool
	MessageID    string
	FallbackUsed MediaType
	Error        error
}

// MediaDispatcher provides capability-aware media sending across platforms.
// It checks platform capabilities and applies fallback strategies when a
// platform doesn't support the requested media type.
type MediaDispatcher struct {
	registry *Registry
}

// NewMediaDispatcher creates a dispatcher backed by the platform registry.
func NewMediaDispatcher(registry *Registry) *MediaDispatcher {
	return &MediaDispatcher{registry: registry}
}

// ErrMissingChatID is returned when no chat_id is provided in media metadata.
var ErrMissingChatID = errors.New("media dispatch: missing chat_id in metadata")

// ErrInvalidPath is returned when a media path contains traversal sequences.
var ErrInvalidPath = errors.New("media dispatch: path contains traversal sequences")

// Dispatch sends a media payload through the given adapter, checking
// capabilities and applying fallbacks as needed.
func (d *MediaDispatcher) Dispatch(ctx context.Context, adapter PlatformAdapter, payload MediaPayload) *MediaDispatchResult {
	if metaChatID(payload.Metadata) == "" {
		return &MediaDispatchResult{Error: ErrMissingChatID}
	}

	if payload.Path != "" && strings.Contains(payload.Path, "..") {
		return &MediaDispatchResult{Error: ErrInvalidPath}
	}

	caps, hasCaps := d.registry.Capabilities(adapter.Platform())
	if !hasCaps {
		caps = PlatformCapabilities{
			SupportsImages:    true,
			SupportsDocuments: true,
		}
	}

	filePath := payload.Path
	if filePath == "" && payload.URL != "" {
		filePath = payload.URL
	}

	switch payload.Type {
	case MediaTypeImage:
		return d.dispatchImage(ctx, adapter, caps, filePath, payload)
	case MediaTypeVideo:
		return d.dispatchVideo(ctx, adapter, caps, filePath, payload)
	case MediaTypeVoice, MediaTypeAudio:
		return d.dispatchVoice(ctx, adapter, caps, filePath, payload)
	case MediaTypeDocument:
		return d.dispatchDocument(ctx, adapter, caps, filePath, payload)
	default:
		return d.dispatchDocument(ctx, adapter, caps, filePath, payload)
	}
}

// DetectMediaType infers the media type from a file path or extension.
func DetectMediaType(path string) MediaType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return MediaTypeImage
	case ".mp4", ".avi", ".mov", ".mkv", ".webm":
		return MediaTypeVideo
	case ".ogg", ".opus", ".wav", ".mp3", ".m4a", ".flac", ".aac":
		return MediaTypeAudio
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".txt", ".csv", ".json", ".xml", ".zip", ".tar", ".gz":
		return MediaTypeDocument
	default:
		return MediaTypeDocument
	}
}

func (d *MediaDispatcher) dispatchImage(ctx context.Context, adapter PlatformAdapter, caps PlatformCapabilities, filePath string, payload MediaPayload) *MediaDispatchResult {
	if caps.SupportsImages {
		res, err := adapter.SendImage(ctx, metaChatID(payload.Metadata), filePath, payload.Caption, payload.Metadata)
		return toDispatchResult(res, err, "")
	}
	// Fallback: send as document.
	if caps.SupportsDocuments {
		slog.Debug("Image fallback to document", "platform", adapter.Platform())
		res, err := adapter.SendDocument(ctx, metaChatID(payload.Metadata), filePath, payload.Metadata)
		return toDispatchResult(res, err, MediaTypeDocument)
	}
	// Last resort: send as text link.
	return d.sendAsLink(ctx, adapter, payload)
}

func (d *MediaDispatcher) dispatchVideo(ctx context.Context, adapter PlatformAdapter, caps PlatformCapabilities, filePath string, payload MediaPayload) *MediaDispatchResult {
	if caps.SupportsVideo {
		// Video uses SendDocument with a video hint in metadata.
		meta := mergeMetadata(payload.Metadata, map[string]string{"media_type": "video"})
		res, err := adapter.SendDocument(ctx, metaChatID(payload.Metadata), filePath, meta)
		return toDispatchResult(res, err, "")
	}
	// Fallback: send as document.
	if caps.SupportsDocuments {
		slog.Debug("Video fallback to document", "platform", adapter.Platform())
		res, err := adapter.SendDocument(ctx, metaChatID(payload.Metadata), filePath, payload.Metadata)
		return toDispatchResult(res, err, MediaTypeDocument)
	}
	return d.sendAsLink(ctx, adapter, payload)
}

func (d *MediaDispatcher) dispatchVoice(ctx context.Context, adapter PlatformAdapter, caps PlatformCapabilities, filePath string, payload MediaPayload) *MediaDispatchResult {
	if caps.SupportsVoice {
		res, err := adapter.SendVoice(ctx, metaChatID(payload.Metadata), filePath, payload.Metadata)
		return toDispatchResult(res, err, "")
	}
	// Fallback: send as document.
	if caps.SupportsDocuments {
		slog.Debug("Voice fallback to document", "platform", adapter.Platform())
		res, err := adapter.SendDocument(ctx, metaChatID(payload.Metadata), filePath, payload.Metadata)
		return toDispatchResult(res, err, MediaTypeDocument)
	}
	return d.sendAsLink(ctx, adapter, payload)
}

func (d *MediaDispatcher) dispatchDocument(ctx context.Context, adapter PlatformAdapter, caps PlatformCapabilities, filePath string, payload MediaPayload) *MediaDispatchResult {
	if caps.SupportsDocuments {
		res, err := adapter.SendDocument(ctx, metaChatID(payload.Metadata), filePath, payload.Metadata)
		return toDispatchResult(res, err, "")
	}
	return d.sendAsLink(ctx, adapter, payload)
}

func (d *MediaDispatcher) sendAsLink(ctx context.Context, adapter PlatformAdapter, payload MediaPayload) *MediaDispatchResult {
	url := payload.URL
	if url == "" {
		url = payload.Path
	}
	text := fmt.Sprintf("[%s] %s", payload.Type, url)
	if payload.Caption != "" {
		text = payload.Caption + "\n" + text
	}
	res, err := adapter.Send(ctx, metaChatID(payload.Metadata), text, payload.Metadata)
	return toDispatchResult(res, err, "link")
}

func toDispatchResult(res *SendResult, err error, fallback interface{}) *MediaDispatchResult {
	r := &MediaDispatchResult{}
	if err != nil {
		r.Error = err
		return r
	}
	if res != nil {
		r.Success = res.Success
		r.MessageID = res.MessageID
		if !res.Success && res.Error != "" {
			r.Error = fmt.Errorf("%s", res.Error)
		}
	}
	switch v := fallback.(type) {
	case MediaType:
		r.FallbackUsed = v
	case string:
		if v != "" {
			r.FallbackUsed = MediaType(v)
		}
	}
	return r
}

func metaChatID(metadata map[string]string) string {
	if metadata == nil {
		return ""
	}
	return metadata["chat_id"]
}

func mergeMetadata(base, extra map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}
