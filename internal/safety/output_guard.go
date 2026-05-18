package safety

import (
	"regexp"
	"strings"
)

type OutputGuard struct {
	leakagePatterns []*regexp.Regexp
	indicatorWords  []string
}

func NewOutputGuard() *OutputGuard {
	return &OutputGuard{
		leakagePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)my\s+(system\s+)?instructions?\s+(are|say|tell|state)`),
			regexp.MustCompile(`(?i)here\s+(are|is)\s+(my|the)\s+(system\s+)?(prompt|instructions?|rules?)`),
			regexp.MustCompile(`(?i)(system\s+prompt|initial\s+instructions?)\s*:`),
			regexp.MustCompile(`(?i)i\s+(was|am)\s+(told|instructed|programmed)\s+to`),
			regexp.MustCompile(`(?i)my\s+(original|initial|core)\s+(instructions?|programming|directives?)`),
			regexp.MustCompile(`(?i)the\s+developer\s+(told|instructed|configured)\s+me`),
		},
		indicatorWords: []string{
			"as instructed in my system prompt",
			"according to my instructions",
			"my hidden instructions",
			"my secret instructions",
			"my initial prompt says",
			"[system]",
			"[INST]",
		},
	}
}

func (og *OutputGuard) Scan(output string, rules []OutputRule) []PatternMatch {
	if output == "" {
		return nil
	}

	var matches []PatternMatch

	for _, pat := range og.leakagePatterns {
		if loc := pat.FindString(output); loc != "" {
			matches = append(matches, PatternMatch{
				Category: "system_prompt_leakage",
				Pattern:  pat.String(),
				Match:    truncateMatch(loc, 120),
				Severity: 8,
			})
		}
	}

	lower := strings.ToLower(output)
	for _, indicator := range og.indicatorWords {
		if strings.Contains(lower, strings.ToLower(indicator)) {
			matches = append(matches, PatternMatch{
				Category: "instruction_following_indicator",
				Pattern:  indicator,
				Match:    indicator,
				Severity: 6,
			})
		}
	}

	for _, rule := range rules {
		if rule.Regex != nil {
			if loc := rule.Regex.FindString(output); loc != "" {
				matches = append(matches, PatternMatch{
					Category: "custom_output_rule",
					Pattern:  rule.Description,
					Match:    truncateMatch(loc, 120),
					Severity: rule.Severity,
				})
			}
		} else if rule.Contains != "" {
			if strings.Contains(lower, strings.ToLower(rule.Contains)) {
				matches = append(matches, PatternMatch{
					Category: "custom_output_rule",
					Pattern:  rule.Description,
					Match:    rule.Contains,
					Severity: rule.Severity,
				})
			}
		}
	}

	return matches
}
