package agent

// StreamHandler dispatches streaming events to registered callbacks.
// Extracted from AIAgent to centralise nil-check logic and enable
// independent testing of callback dispatch.
type StreamHandler struct {
	cb *StreamCallbacks
}

// NewStreamHandler creates a StreamHandler. Pass nil for a no-op handler.
func NewStreamHandler(cb *StreamCallbacks) *StreamHandler {
	return &StreamHandler{cb: cb}
}

// SetCallbacks replaces the underlying callback set.
func (h *StreamHandler) SetCallbacks(cb *StreamCallbacks) {
	h.cb = cb
}

// HasStreamConsumers returns true if any streaming consumer is registered.
func (h *StreamHandler) HasStreamConsumers() bool {
	return h.cb != nil && h.cb.OnStreamDelta != nil
}

// FireStreamDelta fires a text chunk to the stream consumer.
func (h *StreamHandler) FireStreamDelta(text string) {
	if h.cb != nil && h.cb.OnStreamDelta != nil {
		h.cb.OnStreamDelta(text)
	}
}

// FireReasoning fires a reasoning chunk.
func (h *StreamHandler) FireReasoning(text string) {
	if h.cb != nil && h.cb.OnReasoning != nil {
		h.cb.OnReasoning(text)
	}
}

// FireThinking fires a thinking message.
func (h *StreamHandler) FireThinking(msg string) {
	if h.cb != nil && h.cb.OnThinking != nil {
		h.cb.OnThinking(msg)
	}
}

// FireToolProgress fires tool execution progress.
func (h *StreamHandler) FireToolProgress(toolName, argsPreview string) {
	if h.cb != nil && h.cb.OnToolProgress != nil {
		h.cb.OnToolProgress(toolName, argsPreview)
	}
}

// FireToolStart fires when a tool starts executing.
func (h *StreamHandler) FireToolStart(toolName string) {
	if h.cb != nil && h.cb.OnToolStart != nil {
		h.cb.OnToolStart(toolName)
	}
}

// FireToolComplete fires when a tool completes.
func (h *StreamHandler) FireToolComplete(toolName string) {
	if h.cb != nil && h.cb.OnToolComplete != nil {
		h.cb.OnToolComplete(toolName)
	}
}

// FireStep fires on each API step.
func (h *StreamHandler) FireStep(iteration int, prevTools []string) {
	if h.cb != nil && h.cb.OnStep != nil {
		h.cb.OnStep(iteration, prevTools)
	}
}

// FireStatus fires status updates.
func (h *StreamHandler) FireStatus(msg string) {
	if h.cb != nil && h.cb.OnStatus != nil {
		h.cb.OnStatus(msg)
	}
}

// FireError fires the error callback if registered.
func (h *StreamHandler) FireError(err error) {
	if h.cb != nil && h.cb.OnError != nil {
		h.cb.OnError(err)
	}
}

// Clarify fires the clarify callback and returns the user's answer.
// Returns empty string if no clarify handler is registered.
func (h *StreamHandler) Clarify(question string, choices []string) string {
	if h.cb != nil && h.cb.OnClarify != nil {
		return h.cb.OnClarify(question, choices)
	}
	return ""
}
