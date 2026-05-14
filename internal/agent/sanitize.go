package agent

import (
	"regexp"
	"strings"
)

// surrogateRE matches lone surrogate code points (U+D800 through U+DFFF).
// These are invalid in UTF-8 and will crash json.Marshal.
var surrogateRE = regexp.MustCompile(`[\x{D800}-\x{DFFF}]`)

// SanitizeSurrogates replaces lone surrogate code points with U+FFFD
// (replacement character). This is a fast no-op when no surrogates are present.
func SanitizeSurrogates(text string) string {
	if !surrogateRE.MatchString(text) {
		return text
	}
	return surrogateRE.ReplaceAllString(text, "�")
}

// bidiRE matches Unicode bidirectional control characters used in bidi override
// attacks. These characters are invisible but can cause LLMs to misinterpret
// the logical order of text, enabling prompt injection.
//
// Covered ranges:
//
//	U+061C  Arabic Letter Mark
//	U+200E–U+200F  LRM / RLM
//	U+202A–U+202E  LRE, RLE, PDF, LRO, RLO
//	U+2066–U+2069  LRI, RLI, FSI, PDI
var bidiRE = regexp.MustCompile(`[\x{061C}\x{200E}\x{200F}\x{202A}-\x{202E}\x{2066}-\x{2069}]`)

// sanitizeForPrompt strips control characters, Unicode bidi override characters,
// and truncates to maxLen runes.
// Prevents prompt injection when interpolating user content into LLM prompts.
func sanitizeForPrompt(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	// Strip bidi override characters that can mislead LLM token processing.
	if bidiRE.MatchString(s) {
		s = bidiRE.ReplaceAllString(s, "")
	}
	if maxLen > 0 {
		runes := []rune(s)
		if len(runes) > maxLen {
			s = string(runes[:maxLen]) + "..."
		}
	}
	return s
}
