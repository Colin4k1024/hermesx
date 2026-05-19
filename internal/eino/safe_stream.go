package eino

import "strings"

const safeStreamBufferThreshold = 64

// safeStreamWriter buffers streaming chunks and applies redaction before
// emitting to the OnStreamDelta callback. This prevents partial secrets from
// leaking through individual stream events.
//
// Strategy: accumulate content in a buffer. When the buffer exceeds the
// threshold, redact the buffer, emit the safe prefix (all but the last
// `threshold` bytes which might contain a partial secret), and retain the
// tail for the next round. On Flush, redact and emit everything remaining.
type safeStreamWriter struct {
	buf      strings.Builder
	emitted  int
	hook     *RedactionHook
	cb       *StreamCallbacks
	allParts []string
}

func newSafeStreamWriter(hook *RedactionHook, cb *StreamCallbacks) *safeStreamWriter {
	return &safeStreamWriter{hook: hook, cb: cb}
}

func (w *safeStreamWriter) Write(chunk string) {
	w.buf.WriteString(chunk)
	w.allParts = append(w.allParts, chunk)

	if w.buf.Len()-w.emitted >= safeStreamBufferThreshold*2 {
		w.emitSafePrefix()
	}
}

func (w *safeStreamWriter) emitSafePrefix() {
	full := w.buf.String()
	redacted := w.hook.RedactToolOutput(full)

	safeEnd := len(redacted) - safeStreamBufferThreshold
	if safeEnd <= w.emitted {
		return
	}

	delta := redacted[w.emitted:safeEnd]
	w.emitted = safeEnd

	if w.cb != nil && w.cb.OnStreamDelta != nil && delta != "" {
		w.cb.OnStreamDelta(delta)
	}
}

// Flush emits all remaining buffered content (redacted) and returns the full redacted string.
func (w *safeStreamWriter) Flush() string {
	full := w.buf.String()
	redacted := w.hook.RedactToolOutput(full)

	if w.emitted < len(redacted) {
		delta := redacted[w.emitted:]
		if w.cb != nil && w.cb.OnStreamDelta != nil && delta != "" {
			w.cb.OnStreamDelta(delta)
		}
	}

	return redacted
}
