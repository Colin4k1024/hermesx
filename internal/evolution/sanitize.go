package evolution

import (
	"strings"
	"unicode"
)

const maxInsightRunes = 300

// bidiOverrides are Unicode directional override characters used in prompt injection attacks.
var bidiOverrides = []string{"‪", "‫", "‬", "‭", "‮", "⁦", "⁧", "⁨", "⁩"}

// sanitizeInsight strips control characters, bidi overrides, and truncates to maxInsightRunes.
// Applied to every gene insight before prompt injection or storage.
func sanitizeInsight(s string) string {
	for _, b := range bidiOverrides {
		s = strings.ReplaceAll(s, b, "")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxInsightRunes {
		s = string(runes[:maxInsightRunes])
	}
	return s
}
