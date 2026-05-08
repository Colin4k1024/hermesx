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

// sanitizeForPrompt strips control characters and truncates to maxLen runes.
// Prevents prompt injection when interpolating user content into LLM prompts.
func sanitizeForPrompt(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	if maxLen > 0 {
		runes := []rune(s)
		if len(runes) > maxLen {
			s = string(runes[:maxLen]) + "..."
		}
	}
	return s
}
