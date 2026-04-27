package tools

import (
	"regexp"
	"strings"
)

var gitInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`--upload-pack\b`),
	regexp.MustCompile(`--receive-pack\b`),
	regexp.MustCompile(`--exec\s*=`),
	regexp.MustCompile(`-c\s+core\.\w+\s*=`),
	regexp.MustCompile(`-c\s+credential\.helper\s*=`),
	regexp.MustCompile(`-c\s+http\.proxy\s*=`),
	regexp.MustCompile(`ext::`),
	regexp.MustCompile(`--config\s*=\s*http\.`),
	regexp.MustCompile(`-c\s+remote\.\w+\.url\s*=`),
}

// IsGitArgInjection detects git argument injection patterns in a command.
func IsGitArgInjection(command string) bool {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "git ") && !strings.HasPrefix(trimmed, "git\t") {
		return false
	}

	for _, pat := range gitInjectionPatterns {
		if pat.MatchString(command) {
			return true
		}
	}
	return false
}
