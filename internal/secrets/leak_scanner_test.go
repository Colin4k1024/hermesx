package secrets

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestLeakScanner_AWSKeys(t *testing.T) {
	scanner := NewLeakScanner()
	tests := []struct {
		input string
		want  bool
	}{
		{"AKIAIOSFODNN7EXAMPLE", true},
		{"AKIAI0SF0DNN7EXAMPL3", true},
		{"not a key", false},
		{"AKIA too short", false},
	}

	for _, tt := range tests {
		matches := scanner.Scan(tt.input)
		found := false
		for _, m := range matches {
			if m.PatternName == "aws_access_key" {
				found = true
				break
			}
		}
		if found != tt.want {
			t.Errorf("Scan(%q) aws_access_key: got %v, want %v", tt.input, found, tt.want)
		}
	}
}

func TestLeakScanner_GitHubTokens(t *testing.T) {
	scanner := NewLeakScanner()
	tests := []struct {
		input string
		want  string
	}{
		{"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijkl", "github_token_ghp"},
		{"ghs_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijkl", "github_token_ghs"},
	}

	for _, tt := range tests {
		matches := scanner.Scan(tt.input)
		found := false
		for _, m := range matches {
			if m.PatternName == tt.want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Scan(%q) expected pattern %q not found", tt.input, tt.want)
		}
	}
}

func TestLeakScanner_SlackTokens(t *testing.T) {
	scanner := NewLeakScanner()
	// Use fmt.Sprintf to construct test token — avoids GitHub push protection false positive
	input := fmt.Sprintf("token is xoxb-%s-%s-%s", "000000000000", "0000000000000", "TestTokenValueXx")
	matches := scanner.Scan(input)
	found := false
	for _, m := range matches {
		if m.PatternName == "slack_token" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected slack_token match not found")
	}
}

func TestLeakScanner_JWT(t *testing.T) {
	scanner := NewLeakScanner()
	input := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	matches := scanner.Scan(input)
	found := false
	for _, m := range matches {
		if m.PatternName == "jwt_token" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected jwt_token match not found")
	}
}

func TestLeakScanner_DatabaseURLs(t *testing.T) {
	scanner := NewLeakScanner()
	tests := []string{
		"postgres://user:pass@host:5432/db",
		"mysql://root:secret@localhost/mydb",
		"mongodb://admin:pass123@cluster.mongodb.net/test",
	}

	for _, input := range tests {
		matches := scanner.Scan(input)
		found := false
		for _, m := range matches {
			if m.PatternName == "database_url" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Scan(%q) expected database_url match not found", input)
		}
	}
}

func TestLeakScanner_PrivateKeys(t *testing.T) {
	scanner := NewLeakScanner()
	tests := []string{
		"-----BEGIN RSA PRIVATE KEY-----",
		"-----BEGIN EC PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
	}

	for _, input := range tests {
		matches := scanner.Scan(input)
		if len(matches) == 0 {
			t.Errorf("Scan(%q) expected private key match", input)
		}
	}
}

func TestLeakScanner_OpenAIKey(t *testing.T) {
	scanner := NewLeakScanner()
	input := "sk-abcdefghijklmnopqrstuvwxyz123456"
	matches := scanner.Scan(input)
	found := false
	for _, m := range matches {
		if m.PatternName == "openai_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected openai_key match not found")
	}
}

func TestLeakScanner_LiteralSecrets(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"DB_PASSWORD": "my-super-secret-password",
		"API_TOKEN":   "custom_token_value_12345",
	})

	input := "connecting with password my-super-secret-password to database"
	matches := scanner.Scan(input)
	found := false
	for _, m := range matches {
		if m.PatternName == "resolved_secret:DB_PASSWORD" {
			found = true
			if m.Value != "my-s***" {
				t.Errorf("unexpected match value: %q", m.Value)
			}
			break
		}
	}
	if !found {
		t.Error("expected literal secret match not found")
	}
}

func TestLeakScanner_LiteralSecrets_ShortIgnored(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"SHORT": "abc",
	})

	input := "abc is too short to be a secret abc"
	matches := scanner.Scan(input)
	for _, m := range matches {
		if m.PatternName == "resolved_secret:SHORT" {
			t.Error("short literal secrets should be ignored")
		}
	}
}

func TestLeakScanner_Redact(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"MY_KEY": "supersecretvalue1234",
	})

	input := "the key is supersecretvalue1234 and it's secret"
	redacted, matches := scanner.Redact(input)
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if strings.Contains(redacted, "supersecretvalue1234") {
		t.Errorf("redacted output still contains secret: %q", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED:") {
		t.Errorf("redacted output missing redaction marker: %q", redacted)
	}
}

func TestLeakScanner_Redact_AWSKey(t *testing.T) {
	scanner := NewLeakScanner()
	input := "key: AKIAIOSFODNN7EXAMPLE access granted"
	redacted, matches := scanner.Redact(input)
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if strings.Contains(redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("redacted output still contains AWS key")
	}
	if !strings.Contains(redacted, "[REDACTED:aws_access_key]") {
		t.Errorf("expected redaction marker, got: %q", redacted)
	}
}

func TestLeakScanner_NoFalsePositiveOnNormalText(t *testing.T) {
	scanner := NewLeakScanner()
	inputs := []string{
		"Hello, this is a normal message.",
		"The meeting is at 3pm tomorrow.",
		"Please update the documentation.",
		"Function returned error: connection timeout after 30s",
		"Status code: 200 OK",
	}

	for _, input := range inputs {
		matches := scanner.Scan(input)
		highSeverity := 0
		for _, m := range matches {
			if m.Severity == SeverityCritical || m.Severity == SeverityHigh {
				highSeverity++
			}
		}
		if highSeverity > 0 {
			t.Errorf("false positive on %q: %d high/critical matches", input, highSeverity)
		}
	}
}

func TestLeakScanner_AddRemovePattern(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.AddPattern("custom_secret", regexp.MustCompile(`MYSECRET_[A-Z]{10}`), SeverityHigh)

	input := "found MYSECRET_ABCDEFGHIJ in output"
	matches := scanner.Scan(input)
	found := false
	for _, m := range matches {
		if m.PatternName == "custom_secret" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom pattern not detected")
	}

	scanner.RemovePattern("custom_secret")
	matches = scanner.Scan(input)
	for _, m := range matches {
		if m.PatternName == "custom_secret" {
			t.Error("removed pattern still detected")
		}
	}
}

func TestLeakScanner_EmptyInput(t *testing.T) {
	scanner := NewLeakScanner()
	matches := scanner.Scan("")
	if matches != nil {
		t.Error("expected nil matches for empty input")
	}
}

func TestLeakScanner_MultipleMatchesSameText(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"SECRET1": "first_secret_value",
		"SECRET2": "second_secret_value",
	})

	input := "data: first_secret_value and also second_secret_value here"
	matches := scanner.Scan(input)
	foundFirst := false
	foundSecond := false
	for _, m := range matches {
		if m.PatternName == "resolved_secret:SECRET1" {
			foundFirst = true
		}
		if m.PatternName == "resolved_secret:SECRET2" {
			foundSecond = true
		}
	}
	if !foundFirst || !foundSecond {
		t.Errorf("expected both secrets found: first=%v second=%v", foundFirst, foundSecond)
	}
}

func TestLeakScanner_Severity(t *testing.T) {
	scanner := NewLeakScanner()
	tests := []struct {
		input    string
		pattern  string
		severity Severity
	}{
		{"AKIAIOSFODNN7EXAMPLE", "aws_access_key", SeverityCritical},
		{"xoxb-token-value-here", "slack_token", SeverityHigh},
	}

	for _, tt := range tests {
		matches := scanner.Scan(tt.input)
		for _, m := range matches {
			if m.PatternName == tt.pattern {
				if m.Severity != tt.severity {
					t.Errorf("pattern %q severity: got %q, want %q", tt.pattern, m.Severity, tt.severity)
				}
			}
		}
	}
}

func TestLeakScanner_Performance_LargeOutput(t *testing.T) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"SECRET": "needle_in_haystack_secret",
	})

	haystack := strings.Repeat("normal text without any secrets here. ", 10000)
	haystack += "needle_in_haystack_secret"
	haystack += strings.Repeat(" more normal text follows. ", 10000)

	matches := scanner.Scan(haystack)
	found := false
	for _, m := range matches {
		if m.PatternName == "resolved_secret:SECRET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("failed to find secret in large output")
	}
}

func TestLeakScanner_Patterns(t *testing.T) {
	scanner := NewLeakScanner()
	patterns := scanner.Patterns()
	if len(patterns) < 40 {
		t.Errorf("expected at least 40 built-in patterns, got %d", len(patterns))
	}
}

func TestLeakScanner_DetectionRate(t *testing.T) {
	scanner := NewLeakScanner()

	secretSamples := []struct {
		name  string
		value string
	}{
		{"aws", "AKIAIOSFODNN7EXAMPLE"},
		{"github", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijkl"},
		{"slack", "xoxb-123-456-abcdefgh"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.rTCH8cLoGxAm_xw68z-zXVKi9ie6xJn9tnVWjd_9ftE"},
		{"postgres", "postgres://user:pass@host:5432/db"},
		{"private_key", "-----BEGIN RSA PRIVATE KEY-----"},
		{"openai", "sk-abcdefghijklmnopqrstuvwxyz1234"},
		{"gitlab", "glpat-ABCDEFGHIJKLMNOPQRST"},
		{"anthropic", "sk-ant-abcdefghijklmnopqrst"},
	}

	detected := 0
	for _, s := range secretSamples {
		matches := scanner.Scan(s.value)
		if len(matches) > 0 {
			detected++
		} else {
			t.Logf("missed detection for %s: %q", s.name, s.value)
		}
	}

	rate := float64(detected) / float64(len(secretSamples))
	if rate < 0.99 {
		t.Errorf("detection rate %.2f%% below 99%% target", rate*100)
	}
}

func TestAhoCorasick_Basic(t *testing.T) {
	patterns := []string{"he", "she", "his", "hers"}
	ac := NewAhoCorasick(patterns)

	matches := ac.Search("ahishers")
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}

	foundPatterns := make(map[int]bool)
	for _, m := range matches {
		foundPatterns[m.PatternIndex] = true
	}

	for i := range patterns {
		if !foundPatterns[i] {
			t.Errorf("pattern %q not found", patterns[i])
		}
	}
}

func TestAhoCorasick_NoMatch(t *testing.T) {
	patterns := []string{"xyz", "abc"}
	ac := NewAhoCorasick(patterns)
	matches := ac.Search("hello world")
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestAhoCorasick_Overlapping(t *testing.T) {
	patterns := []string{"ab", "bc", "abc"}
	ac := NewAhoCorasick(patterns)
	matches := ac.Search("abc")
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches for overlapping patterns, got %d", len(matches))
	}
}

func BenchmarkLeakScanner_Scan(b *testing.B) {
	scanner := NewLeakScanner()
	scanner.SetLiteralSecrets(map[string]string{
		"SECRET1": "abcdefghijklmnopqrstuvwxyz1234567890",
		"SECRET2": "ZYXWVUTSRQPONMLKJIHGFEDCBA0987654321",
	})

	text := strings.Repeat("normal output from a tool with no secrets. ", 1000)
	text += "AKIAIOSFODNN7EXAMPLE"
	text += strings.Repeat(" more output. ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Scan(text)
	}
}

func BenchmarkLeakScanner_Redact(b *testing.B) {
	scanner := NewLeakScanner()
	text := fmt.Sprintf("key=%s token=%s",
		"AKIAIOSFODNN7EXAMPLE",
		"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijkl")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Redact(text)
	}
}

func BenchmarkAhoCorasick_Search(b *testing.B) {
	patterns := make([]string, 100)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("pattern_%d_value", i)
	}
	ac := NewAhoCorasick(patterns)
	text := strings.Repeat("some text without matches. ", 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ac.Search(text)
	}
}
