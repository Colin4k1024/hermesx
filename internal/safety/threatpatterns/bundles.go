// Package threatpatterns provides named, versioned collections of threat
// detection patterns that can be loaded into a safety.PatternRegistry.
//
// Each bundle targets a specific attack surface:
//
//   - PromptInjection   — instruction-override attempts
//   - RoleHijack        — persona / role reassignment
//   - PromptExtraction  — system-prompt leakage triggers
//   - DelimiterInjection — structural context-boundary forgery
//   - EncodingAttack    — obfuscation via base64, hex, rot13 …
//   - SafetyBypass      — attempts to disable safety controls
//   - IndirectInjection — third-party / tool-mediated injection
//
// Bundles are pure data (no compiled regexp); callers compile patterns when
// loading them into a PatternRegistry to defer any regexp errors.
package threatpatterns

// Pattern is a single threat detection rule.
// Regex must be a valid Go regexp string; compilation is deferred to the caller.
type Pattern struct {
	Name     string
	Category string
	Regex    string
	Severity int
}

// Bundle is a named, versioned collection of threat patterns.
type Bundle struct {
	Name     string
	Version  string
	Patterns []Pattern
}

// PromptInjection returns patterns that detect attempts to override or replace
// the model's original instructions.
func PromptInjection() Bundle {
	return Bundle{
		Name:    "prompt_injection",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "ignore_previous", Category: "instruction_override", Regex: `(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions|prompts?|context)`, Severity: 8},
			{Name: "disregard_instructions", Category: "instruction_override", Regex: `(?i)disregard\s+(your|all|any|the)?\s*(instructions|rules|guidelines|constraints)`, Severity: 8},
			{Name: "new_instructions", Category: "instruction_override", Regex: `(?i)(new|updated|revised|override)\s+instructions?\s*:`, Severity: 7},
			{Name: "forget_everything", Category: "instruction_override", Regex: `(?i)forget\s+(everything|all|what)\s+(you|about|i)`, Severity: 8},
			{Name: "do_not_follow", Category: "instruction_override", Regex: `(?i)do\s+not\s+follow\s+(your|any|the)\s+(previous|original|initial)`, Severity: 8},
		},
	}
}

// RoleHijack returns patterns that detect attempts to reassign the model's
// active persona or role.
func RoleHijack() Bundle {
	return Bundle{
		Name:    "role_hijack",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "you_are_now", Category: "role_hijack", Regex: `(?i)you\s+are\s+now\s+(a|an|the|my)\s+\w+`, Severity: 7},
			{Name: "act_as", Category: "role_hijack", Regex: `(?i)(act|behave|pretend|roleplay)\s+(as|like)\s+(a|an|the|my)?\s*\w+`, Severity: 6},
			{Name: "new_persona", Category: "role_hijack", Regex: `(?i)(switch|change)\s+(to|into)\s+(a|an)?\s*(new|different)\s+(role|persona|character|mode)`, Severity: 7},
			{Name: "jailbreak_mode", Category: "role_hijack", Regex: `(?i)(DAN|developer|god|admin|root|sudo)\s+mode`, Severity: 9},
			{Name: "unrestricted", Category: "role_hijack", Regex: `(?i)(no\s+restrictions?|without\s+(any\s+)?limitations?|unrestricted\s+mode)`, Severity: 8},
		},
	}
}

// PromptExtraction returns patterns that detect attempts to reveal system
// prompts or internal instructions.
func PromptExtraction() Bundle {
	return Bundle{
		Name:    "prompt_extraction",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "system_prompt_reveal", Category: "prompt_extraction", Regex: `(?i)(reveal|show|display|print|output|repeat|echo)\s+(your|the|my)?\s*(system\s+)?(prompt|instructions?|rules?|guidelines?)`, Severity: 9},
			{Name: "what_are_instructions", Category: "prompt_extraction", Regex: `(?i)what\s+(are|were)\s+(your|the)\s+(system\s+)?(instructions?|rules?|prompt|guidelines?)`, Severity: 7},
			{Name: "above_text", Category: "prompt_extraction", Regex: `(?i)(repeat|copy|paste|write)\s+(the\s+)?(text|content|message)\s+(above|before|preceding)`, Severity: 7},
			{Name: "verbatim_repeat", Category: "prompt_extraction", Regex: `(?i)repeat\s+(everything|all|verbatim|word\s+for\s+word)`, Severity: 8},
		},
	}
}

// DelimiterInjection returns patterns detecting structural delimiters used to
// forge context boundaries or inject pseudo-system messages.
func DelimiterInjection() Bundle {
	return Bundle{
		Name:    "delimiter_injection",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "delimiter_close", Category: "delimiter_injection", Regex: `(?i)(</?system>|</?instruction>|\[/?INST\]|<\|im_end\|>|<\|im_start\|>)`, Severity: 6},
			{Name: "xml_system_tag", Category: "delimiter_injection", Regex: `(?i)<\s*/?\s*(system|assistant|function_call|tool_result)\s*>`, Severity: 8},
			{Name: "markdown_role", Category: "delimiter_injection", Regex: `(?i)###\s*(system|assistant|function)\s*\n`, Severity: 6},
		},
	}
}

// EncodingAttack returns patterns detecting encoding-based obfuscation attempts
// intended to bypass text-level safety checks.
func EncodingAttack() Bundle {
	return Bundle{
		Name:    "encoding_attack",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "base64_instruct", Category: "encoding_attack", Regex: `(?i)(decode|execute|run|eval)\s+(this\s+)?base64`, Severity: 7},
			{Name: "hex_instruct", Category: "encoding_attack", Regex: `(?i)(decode|execute|run)\s+(this\s+)?hex(adecimal)?`, Severity: 7},
			{Name: "rot13_instruct", Category: "encoding_attack", Regex: `(?i)(decode|apply|use)\s+rot13`, Severity: 6},
		},
	}
}

// SafetyBypass returns patterns detecting explicit requests to disable or
// circumvent safety controls.
func SafetyBypass() Bundle {
	return Bundle{
		Name:    "safety_bypass",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "ignore_safety", Category: "safety_bypass", Regex: `(?i)(bypass|disable|ignore|skip|override)\s+(the\s+)?(safety|content|ethical|security)\s*(filter|check|guard|policy|system)`, Severity: 9},
			{Name: "hypothetical_scenario", Category: "safety_bypass", Regex: `(?i)(hypothetical(ly)?|theoretically|for\s+(research|educational)\s+purposes?)\s*,?\s*(how|what|can|would)`, Severity: 4},
			{Name: "fictional_context", Category: "safety_bypass", Regex: `(?i)in\s+(a|this)\s+(fictional|imaginary|hypothetical|alternate)\s+(world|universe|scenario|story)`, Severity: 4},
		},
	}
}

// IndirectInjection returns patterns detecting third-party or tool-mediated
// prompt injection (e.g., injections embedded in web pages or tool results).
func IndirectInjection() Bundle {
	return Bundle{
		Name:    "indirect_injection",
		Version: "1.0.0",
		Patterns: []Pattern{
			{Name: "indirect_injection_marker", Category: "indirect_injection", Regex: `(?i)(IMPORTANT|ATTENTION|NOTE|WARNING)\s*:?\s*(ignore|disregard|override|forget)\s`, Severity: 8},
			{Name: "hidden_instruction", Category: "indirect_injection", Regex: `(?i)(hidden|secret|embedded)\s+(instruction|command|directive|message)`, Severity: 7},
			{Name: "tool_result_inject", Category: "indirect_injection", Regex: `(?i)(as\s+the\s+tool|tool\s+says?|tool\s+result)\s*:.*ignore`, Severity: 8},
		},
	}
}

// All returns every built-in bundle in recommended detection-priority order:
// high-impact injection attacks first, subtler bypass patterns last.
func All() []Bundle {
	return []Bundle{
		PromptInjection(),
		RoleHijack(),
		PromptExtraction(),
		DelimiterInjection(),
		EncodingAttack(),
		SafetyBypass(),
		IndirectInjection(),
	}
}
