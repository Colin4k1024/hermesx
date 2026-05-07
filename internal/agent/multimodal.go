package agent

import (
	"log/slog"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
)

// MultimodalRouter determines whether to route a request through the vision
// auxiliary client when the current model lacks vision support.
type MultimodalRouter struct {
	Enabled bool
}

// DefaultMultimodalRouter returns a router with sensible defaults.
func DefaultMultimodalRouter() *MultimodalRouter {
	return &MultimodalRouter{Enabled: true}
}

// messagesContainImages returns true if any message in the slice has image URLs.
func messagesContainImages(messages []llm.Message) bool {
	for i := range messages {
		if len(messages[i].ImageURLs) > 0 {
			return true
		}
	}
	return false
}

// NeedsVisionRouting returns true if the request contains images and the
// current model does not support vision.
func (r *MultimodalRouter) NeedsVisionRouting(messages []llm.Message, model string) bool {
	if !r.Enabled {
		return false
	}
	if !messagesContainImages(messages) {
		return false
	}
	meta := llm.GetModelMeta(model)
	if meta.SupportsVision {
		return false
	}
	slog.Debug("Multimodal routing triggered",
		"model", model,
		"supports_vision", false,
	)
	return true
}

// visionClientForRequest returns the vision auxiliary client if available and
// the request needs vision routing, otherwise nil.
func (a *AIAgent) visionClientForRequest(messages []llm.Message) *llm.Client {
	if a.multimodalRouter == nil || a.auxiliaryClient == nil {
		return nil
	}
	if !a.multimodalRouter.NeedsVisionRouting(messages, a.model) {
		return nil
	}
	vc := a.auxiliaryClient.VisionClient()
	if vc == nil {
		slog.Warn("Vision routing needed but no vision auxiliary client configured")
		return nil
	}
	slog.Info("Routing to vision model",
		"primary_model", a.model,
		"vision_model", vc.Model(),
	)
	return vc
}
