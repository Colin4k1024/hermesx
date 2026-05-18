package safety

import (
	"regexp"
	"sync"
)

type PatternEntry struct {
	Name     string
	Category string
	Regex    *regexp.Regexp
	Severity int
}

type PatternRegistry struct {
	mu       sync.RWMutex
	patterns []PatternEntry
}

var defaultRegistry = NewPatternRegistry()

func DefaultPatternRegistry() *PatternRegistry {
	return defaultRegistry
}

func NewPatternRegistry() *PatternRegistry {
	pr := &PatternRegistry{}
	pr.loadDefaults()
	return pr
}

func (pr *PatternRegistry) Patterns() []PatternEntry {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	out := make([]PatternEntry, len(pr.patterns))
	copy(out, pr.patterns)
	return out
}

func (pr *PatternRegistry) Reload(patterns []PatternEntry) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.patterns = patterns
}

func (pr *PatternRegistry) Add(entries ...PatternEntry) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.patterns = append(pr.patterns, entries...)
}

func (pr *PatternRegistry) loadDefaults() {
	pr.patterns = []PatternEntry{
		{Name: "ignore_previous", Category: "instruction_override", Regex: regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions|prompts?|context)`), Severity: 8},
		{Name: "disregard_instructions", Category: "instruction_override", Regex: regexp.MustCompile(`(?i)disregard\s+(your|all|any|the)?\s*(instructions|rules|guidelines|constraints)`), Severity: 8},
		{Name: "new_instructions", Category: "instruction_override", Regex: regexp.MustCompile(`(?i)(new|updated|revised|override)\s+instructions?\s*:`), Severity: 7},
		{Name: "forget_everything", Category: "instruction_override", Regex: regexp.MustCompile(`(?i)forget\s+(everything|all|what)\s+(you|about|i)`), Severity: 8},
		{Name: "do_not_follow", Category: "instruction_override", Regex: regexp.MustCompile(`(?i)do\s+not\s+follow\s+(your|any|the)\s+(previous|original|initial)`), Severity: 8},

		{Name: "you_are_now", Category: "role_hijack", Regex: regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an|the|my)\s+\w+`), Severity: 7},
		{Name: "act_as", Category: "role_hijack", Regex: regexp.MustCompile(`(?i)(act|behave|pretend|roleplay)\s+(as|like)\s+(a|an|the|my)?\s*\w+`), Severity: 6},
		{Name: "new_persona", Category: "role_hijack", Regex: regexp.MustCompile(`(?i)(switch|change)\s+(to|into)\s+(a|an)?\s*(new|different)\s+(role|persona|character|mode)`), Severity: 7},
		{Name: "jailbreak_mode", Category: "role_hijack", Regex: regexp.MustCompile(`(?i)(DAN|developer|god|admin|root|sudo)\s+mode`), Severity: 9},
		{Name: "unrestricted", Category: "role_hijack", Regex: regexp.MustCompile(`(?i)(no\s+restrictions?|without\s+(any\s+)?limitations?|unrestricted\s+mode)`), Severity: 8},

		{Name: "system_prompt_reveal", Category: "prompt_extraction", Regex: regexp.MustCompile(`(?i)(reveal|show|display|print|output|repeat|echo)\s+(your|the|my)?\s*(system\s+)?(prompt|instructions?|rules?|guidelines?)`), Severity: 9},
		{Name: "what_are_instructions", Category: "prompt_extraction", Regex: regexp.MustCompile(`(?i)what\s+(are|were)\s+(your|the)\s+(system\s+)?(instructions?|rules?|prompt|guidelines?)`), Severity: 7},
		{Name: "above_text", Category: "prompt_extraction", Regex: regexp.MustCompile(`(?i)(repeat|copy|paste|write)\s+(the\s+)?(text|content|message)\s+(above|before|preceding)`), Severity: 7},
		{Name: "verbatim_repeat", Category: "prompt_extraction", Regex: regexp.MustCompile(`(?i)repeat\s+(everything|all|verbatim|word\s+for\s+word)`), Severity: 8},

		{Name: "delimiter_close", Category: "delimiter_injection", Regex: regexp.MustCompile("(?i)(</?system>|</?instruction>|\\[/?INST\\]|<\\|im_end\\|>|<\\|im_start\\|>)"), Severity: 6},
		{Name: "xml_system_tag", Category: "delimiter_injection", Regex: regexp.MustCompile(`(?i)<\s*/?\s*(system|assistant|function_call|tool_result)\s*>`), Severity: 8},
		{Name: "markdown_role", Category: "delimiter_injection", Regex: regexp.MustCompile("(?i)###\\s*(system|assistant|function)\\s*\\n"), Severity: 6},

		{Name: "base64_instruct", Category: "encoding_attack", Regex: regexp.MustCompile(`(?i)(decode|execute|run|eval)\s+(this\s+)?base64`), Severity: 7},
		{Name: "hex_instruct", Category: "encoding_attack", Regex: regexp.MustCompile(`(?i)(decode|execute|run)\s+(this\s+)?hex(adecimal)?`), Severity: 7},
		{Name: "rot13_instruct", Category: "encoding_attack", Regex: regexp.MustCompile(`(?i)(decode|apply|use)\s+rot13`), Severity: 6},

		{Name: "ignore_safety", Category: "safety_bypass", Regex: regexp.MustCompile(`(?i)(bypass|disable|ignore|skip|override)\s+(the\s+)?(safety|content|ethical|security)\s*(filter|check|guard|policy|system)`), Severity: 9},
		{Name: "hypothetical_scenario", Category: "safety_bypass", Regex: regexp.MustCompile(`(?i)(hypothetical(ly)?|theoretically|for\s+(research|educational)\s+purposes?)\s*,?\s*(how|what|can|would)`), Severity: 4},
		{Name: "fictional_context", Category: "safety_bypass", Regex: regexp.MustCompile(`(?i)in\s+(a|this)\s+(fictional|imaginary|hypothetical|alternate)\s+(world|universe|scenario|story)`), Severity: 4},

		{Name: "indirect_injection_marker", Category: "indirect_injection", Regex: regexp.MustCompile(`(?i)(IMPORTANT|ATTENTION|NOTE|WARNING)\s*:?\s*(ignore|disregard|override|forget)\s`), Severity: 8},
		{Name: "hidden_instruction", Category: "indirect_injection", Regex: regexp.MustCompile(`(?i)(hidden|secret|embedded)\s+(instruction|command|directive|message)`), Severity: 7},
		{Name: "tool_result_inject", Category: "indirect_injection", Regex: regexp.MustCompile(`(?i)(as\s+the\s+tool|tool\s+says?|tool\s+result)\s*:.*ignore`), Severity: 8},
	}
}
