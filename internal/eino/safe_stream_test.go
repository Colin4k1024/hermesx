package eino

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Colin4k1024/hermesx/internal/secrets"
)

func TestSafeStreamWriter_RedactsSecretsInChunks(t *testing.T) {
	scanner := secrets.NewLeakScanner()
	hook := NewRedactionHook(scanner)

	var deltas []string
	cb := &StreamCallbacks{
		OnStreamDelta: func(text string) {
			deltas = append(deltas, text)
		},
	}

	sw := newSafeStreamWriter(hook, cb)

	// Simulate secret arriving across chunks
	sw.Write("Here is the key: AKIA")
	sw.Write("IOSFODNN7EXAMPLE and more text after that to trigger emission buffer threshold padding")
	result := sw.Flush()

	if result == "Here is the key: AKIAIOSFODNN7EXAMPLE and more text after that to trigger emission buffer threshold padding" {
		t.Error("expected secret to be redacted in final output")
	}

	// Verify callbacks received redacted content
	combined := ""
	for _, d := range deltas {
		combined += d
	}
	if combined != result {
		t.Errorf("callback deltas combined should equal final result\ngot:  %q\nwant: %q", combined, result)
	}
}

func TestSafeStreamWriter_NoSecrets(t *testing.T) {
	scanner := secrets.NewLeakScanner()
	hook := NewRedactionHook(scanner)

	var deltas []string
	cb := &StreamCallbacks{
		OnStreamDelta: func(text string) {
			deltas = append(deltas, text)
		},
	}

	sw := newSafeStreamWriter(hook, cb)
	sw.Write("Hello ")
	sw.Write("world, this is a perfectly safe message with no secrets at all and enough length to trigger buffer")
	result := sw.Flush()

	expected := "Hello world, this is a perfectly safe message with no secrets at all and enough length to trigger buffer"
	if result != expected {
		t.Errorf("expected passthrough, got %q", result)
	}

	combined := ""
	for _, d := range deltas {
		combined += d
	}
	if combined != result {
		t.Errorf("callback total should equal result\ngot:  %q\nwant: %q", combined, result)
	}
}

func TestSafeStreamWriter_NilCallbacks(t *testing.T) {
	scanner := secrets.NewLeakScanner()
	hook := NewRedactionHook(scanner)

	sw := newSafeStreamWriter(hook, nil)
	sw.Write("content with AKIAIOSFODNN7EXAMPLE secret and padding text for buffer")
	result := sw.Flush()

	if result == "content with AKIAIOSFODNN7EXAMPLE secret and padding text for buffer" {
		t.Error("expected redaction even without callbacks")
	}
}

func TestSafeStreamWriter_DoesNotSplitUTF8Runes(t *testing.T) {
	hook := NewRedactionHook(nil)

	var deltas []string
	cb := &StreamCallbacks{
		OnStreamDelta: func(text string) {
			if !utf8.ValidString(text) {
				t.Fatalf("stream delta should be valid UTF-8, got %q", text)
			}
			deltas = append(deltas, text)
		},
	}

	msg := strings.Repeat("a", safeStreamBufferThreshold*2) + "海" + strings.Repeat("b", safeStreamBufferThreshold-2)
	split := len(msg) - safeStreamBufferThreshold
	if split <= 0 || split >= len(msg) || utf8.RuneStart(msg[split]) {
		t.Fatal("test setup failed to place safe split inside a UTF-8 rune")
	}

	sw := newSafeStreamWriter(hook, cb)
	sw.Write(msg)
	result := sw.Flush()

	combined := strings.Join(deltas, "")
	if !utf8.ValidString(combined) {
		t.Fatalf("combined stream output should be valid UTF-8, got %q", combined)
	}
	if result != msg {
		t.Errorf("expected passthrough result\ngot:  %q\nwant: %q", result, msg)
	}
	if combined != result {
		t.Errorf("callback total should equal result\ngot:  %q\nwant: %q", combined, result)
	}
}

func TestSafeStreamWriter_NilScanner(t *testing.T) {
	hook := NewRedactionHook(nil)

	var deltas []string
	cb := &StreamCallbacks{
		OnStreamDelta: func(text string) {
			deltas = append(deltas, text)
		},
	}

	sw := newSafeStreamWriter(hook, cb)
	sw.Write("passthrough AKIAIOSFODNN7EXAMPLE content")
	result := sw.Flush()

	// nil scanner means no redaction
	if result != "passthrough AKIAIOSFODNN7EXAMPLE content" {
		t.Errorf("nil scanner should passthrough, got %q", result)
	}
}
