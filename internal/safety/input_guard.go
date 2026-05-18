package safety

import (
	"strings"
)

type InputGuard struct {
	registry *PatternRegistry
}

func NewInputGuard() *InputGuard {
	return &InputGuard{
		registry: DefaultPatternRegistry(),
	}
}

func (ig *InputGuard) Scan(text string, customPatterns []InputPattern) []PatternMatch {
	if text == "" {
		return nil
	}

	var matches []PatternMatch

	for _, entry := range ig.registry.Patterns() {
		if loc := entry.Regex.FindString(text); loc != "" {
			matches = append(matches, PatternMatch{
				Category: entry.Category,
				Pattern:  entry.Name,
				Match:    truncateMatch(loc, 120),
				Severity: entry.Severity,
			})
		}
	}

	for _, cp := range customPatterns {
		if cp.Regex == nil {
			if strings.Contains(strings.ToLower(text), strings.ToLower(cp.Text)) {
				matches = append(matches, PatternMatch{
					Category: "custom",
					Pattern:  cp.Text,
					Match:    cp.Text,
					Severity: cp.Severity,
				})
			}
			continue
		}
		if loc := cp.Regex.FindString(text); loc != "" {
			matches = append(matches, PatternMatch{
				Category: "custom",
				Pattern:  cp.Text,
				Match:    truncateMatch(loc, 120),
				Severity: cp.Severity,
			})
		}
	}

	return matches
}

func truncateMatch(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
