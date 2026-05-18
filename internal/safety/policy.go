package safety

import "regexp"

type PolicyMode string

const (
	ModeEnforce  PolicyMode = "enforce"
	ModeLogOnly  PolicyMode = "log_only"
	ModeDisabled PolicyMode = "disabled"
)

type InputPattern struct {
	Text     string
	Regex    *regexp.Regexp
	Severity int
}

type OutputRule struct {
	Description string
	Contains    string
	Regex       *regexp.Regexp
	Severity    int
}

type Policy struct {
	ID            string
	TenantID      string
	Mode          PolicyMode
	InputPatterns []InputPattern
	OutputRules   []OutputRule
}

func DefaultPolicy() Policy {
	return Policy{
		Mode:          ModeLogOnly,
		InputPatterns: nil,
		OutputRules:   nil,
	}
}
