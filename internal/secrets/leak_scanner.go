package secrets

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type PatternEntry struct {
	Name     string
	Pattern  *regexp.Regexp
	Severity Severity
}

type LeakMatch struct {
	PatternName string
	Value       string // redacted form: first 4 chars + "***"
	Position    int
	Length      int // original match length in bytes
	Severity    Severity
}

func redactValue(raw string) string {
	if len(raw) <= 4 {
		return "***"
	}
	return raw[:4] + "***"
}

type LeakScanner struct {
	mu              sync.RWMutex
	patterns        []PatternEntry
	literalSecrets  map[string]string
	regexVersion    int
	cachedAutomaton *AhoCorasick
	cachedVersion   int
	cachedLiterals  []string
}

func NewLeakScanner() *LeakScanner {
	s := &LeakScanner{
		literalSecrets: make(map[string]string),
	}
	s.loadBuiltinPatterns()
	return s
}

func (s *LeakScanner) loadBuiltinPatterns() {
	builtins := []PatternEntry{
		{Name: "aws_access_key", Pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), Severity: SeverityCritical},
		{Name: "github_token_ghp", Pattern: regexp.MustCompile(`ghp_[A-Za-z0-9_]{36,}`), Severity: SeverityCritical},
		{Name: "github_token_ghs", Pattern: regexp.MustCompile(`ghs_[A-Za-z0-9_]{36,}`), Severity: SeverityCritical},
		{Name: "slack_token", Pattern: regexp.MustCompile(`xox[baprs]-[A-Za-z0-9\-]+`), Severity: SeverityHigh},
		{Name: "jwt_token", Pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`), Severity: SeverityHigh},
		{Name: "database_url", Pattern: regexp.MustCompile(`(postgres|mysql|mongodb)://[^\s]+`), Severity: SeverityCritical},
		{Name: "private_key", Pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |)PRIVATE KEY-----`), Severity: SeverityCritical},
		{Name: "openai_key", Pattern: regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`), Severity: SeverityCritical},
		{Name: "stripe_key", Pattern: regexp.MustCompile(`sk_live_[A-Za-z0-9]{24,}`), Severity: SeverityCritical},
		{Name: "stripe_test_key", Pattern: regexp.MustCompile(`sk_test_[A-Za-z0-9]{24,}`), Severity: SeverityMedium},
		{Name: "google_api_key", Pattern: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), Severity: SeverityHigh},
		{Name: "heroku_api_key", Pattern: regexp.MustCompile(`[hH]eroku.*[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}`), Severity: SeverityHigh},
		{Name: "mailgun_key", Pattern: regexp.MustCompile(`key-[0-9a-zA-Z]{32}`), Severity: SeverityHigh},
		{Name: "twilio_key", Pattern: regexp.MustCompile(`SK[0-9a-fA-F]{32}`), Severity: SeverityHigh},
		{Name: "sendgrid_key", Pattern: regexp.MustCompile(`SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`), Severity: SeverityHigh},
		{Name: "npm_token", Pattern: regexp.MustCompile(`npm_[A-Za-z0-9]{36}`), Severity: SeverityHigh},
		{Name: "pypi_token", Pattern: regexp.MustCompile(`pypi-[A-Za-z0-9_-]{50,}`), Severity: SeverityHigh},
		{Name: "discord_token", Pattern: regexp.MustCompile(`[MN][A-Za-z\d]{23,}\.[\w-]{6}\.[\w-]{27}`), Severity: SeverityHigh},
		{Name: "azure_storage_key", Pattern: regexp.MustCompile(`DefaultEndpointsProtocol=https;AccountName=[^;]+;AccountKey=[A-Za-z0-9+/=]{88}`), Severity: SeverityCritical},
		{Name: "gcp_service_account", Pattern: regexp.MustCompile(`"type"\s*:\s*"service_account"`), Severity: SeverityHigh},
		{Name: "basic_auth_url", Pattern: regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s]+`), Severity: SeverityHigh},
		{Name: "password_assignment", Pattern: regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']{8,}["']`), Severity: SeverityMedium},
		{Name: "bearer_token", Pattern: regexp.MustCompile(`[Bb]earer\s+[A-Za-z0-9_\-.~+/]+=*`), Severity: SeverityMedium},
		{Name: "ssh_private_key", Pattern: regexp.MustCompile(`-----BEGIN OPENSSH PRIVATE KEY-----`), Severity: SeverityCritical},
		{Name: "aws_secret_key", Pattern: regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*[A-Za-z0-9/+=]{40}`), Severity: SeverityCritical},
		{Name: "gitlab_token", Pattern: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`), Severity: SeverityCritical},
		{Name: "firebase_key", Pattern: regexp.MustCompile(`AAAA[A-Za-z0-9_-]{7}:[A-Za-z0-9_-]{140}`), Severity: SeverityHigh},
		{Name: "slack_webhook", Pattern: regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`), Severity: SeverityHigh},
		{Name: "telegram_bot_token", Pattern: regexp.MustCompile(`[0-9]{8,10}:[A-Za-z0-9_-]{35}`), Severity: SeverityHigh},
		{Name: "shopify_token", Pattern: regexp.MustCompile(`shpat_[a-fA-F0-9]{32}`), Severity: SeverityHigh},
		{Name: "databricks_token", Pattern: regexp.MustCompile(`dapi[a-f0-9]{32}`), Severity: SeverityHigh},
		{Name: "vault_token", Pattern: regexp.MustCompile(`hvs\.[A-Za-z0-9_-]{24,}`), Severity: SeverityCritical},
		{Name: "anthropic_key", Pattern: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`), Severity: SeverityCritical},
		{Name: "huggingface_token", Pattern: regexp.MustCompile(`hf_[A-Za-z0-9]{34,}`), Severity: SeverityHigh},
		{Name: "digitalocean_token", Pattern: regexp.MustCompile(`dop_v1_[a-f0-9]{64}`), Severity: SeverityHigh},
		{Name: "vercel_token", Pattern: regexp.MustCompile(`[A-Za-z0-9]{24}`), Severity: SeverityLow},
		{Name: "supabase_key", Pattern: regexp.MustCompile(`sbp_[a-f0-9]{40}`), Severity: SeverityHigh},
		{Name: "planetscale_token", Pattern: regexp.MustCompile(`pscale_tkn_[A-Za-z0-9_-]{43}`), Severity: SeverityHigh},
		{Name: "linear_api_key", Pattern: regexp.MustCompile(`lin_api_[A-Za-z0-9]{40}`), Severity: SeverityHigh},
		{Name: "cloudflare_api_key", Pattern: regexp.MustCompile(`[a-f0-9]{37}`), Severity: SeverityLow},
		{Name: "datadog_api_key", Pattern: regexp.MustCompile(`dd[a-z]{1}_[A-Za-z0-9]{32,40}`), Severity: SeverityHigh},
		{Name: "fastly_api_key", Pattern: regexp.MustCompile(`[A-Za-z0-9_-]{32}`), Severity: SeverityLow},
		{Name: "github_app_token", Pattern: regexp.MustCompile(`ghu_[A-Za-z0-9]{36}`), Severity: SeverityCritical},
		{Name: "github_refresh_token", Pattern: regexp.MustCompile(`ghr_[A-Za-z0-9]{36}`), Severity: SeverityCritical},
		{Name: "okta_token", Pattern: regexp.MustCompile(`00[A-Za-z0-9_-]{40}`), Severity: SeverityHigh},
		{Name: "confluent_key", Pattern: regexp.MustCompile(`[A-Z0-9]{16}`), Severity: SeverityLow},
		{Name: "age_secret_key", Pattern: regexp.MustCompile(`AGE-SECRET-KEY-[A-Z0-9]{59}`), Severity: SeverityCritical},
		{Name: "doppler_token", Pattern: regexp.MustCompile(`dp\.st\.[a-z0-9_-]+\.[A-Za-z0-9]{40,44}`), Severity: SeverityHigh},
		{Name: "grafana_api_key", Pattern: regexp.MustCompile(`eyJrIjoi[A-Za-z0-9_-]{36,}=`), Severity: SeverityHigh},
		{Name: "postman_api_key", Pattern: regexp.MustCompile(`PMAK-[A-Za-z0-9]{24}-[A-Za-z0-9]{34}`), Severity: SeverityHigh},
		{Name: "connection_string_password", Pattern: regexp.MustCompile(`(?i)(connection.*password|password.*=)[^\s;]{8,}`), Severity: SeverityHigh},
	}
	s.patterns = builtins
}

func (s *LeakScanner) AddPattern(name string, pattern *regexp.Regexp, severity Severity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns = append(s.patterns, PatternEntry{Name: name, Pattern: pattern, Severity: severity})
	s.regexVersion++
}

func (s *LeakScanner) RemovePattern(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.patterns {
		if p.Name == name {
			s.patterns = append(s.patterns[:i], s.patterns[i+1:]...)
			s.regexVersion++
			return
		}
	}
}

func (s *LeakScanner) SetLiteralSecrets(secrets map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.literalSecrets = make(map[string]string, len(secrets))
	for k, v := range secrets {
		if len(v) >= 4 {
			s.literalSecrets[k] = v
		}
	}
	s.regexVersion++
}

func (s *LeakScanner) Patterns() []PatternEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PatternEntry, len(s.patterns))
	copy(out, s.patterns)
	return out
}

func (s *LeakScanner) Scan(text string) []LeakMatch {
	if len(text) == 0 {
		return nil
	}

	var matches []LeakMatch

	s.mu.RLock()
	patterns := make([]PatternEntry, len(s.patterns))
	copy(patterns, s.patterns)
	literals := make(map[string]string, len(s.literalSecrets))
	for k, v := range s.literalSecrets {
		literals[k] = v
	}
	version := s.regexVersion
	s.mu.RUnlock()

	for _, p := range patterns {
		locs := p.Pattern.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matches = append(matches, LeakMatch{
				PatternName: p.Name,
				Value:       redactValue(text[loc[0]:loc[1]]),
				Position:    loc[0],
				Length:      loc[1] - loc[0],
				Severity:    p.Severity,
			})
		}
	}

	if len(literals) > 0 {
		matches = append(matches, s.scanLiterals(text, literals, version)...)
	}

	return matches
}

func (s *LeakScanner) scanLiterals(text string, literals map[string]string, version int) []LeakMatch {
	s.mu.RLock()
	automaton := s.cachedAutomaton
	cachedVersion := s.cachedVersion
	cachedLiterals := s.cachedLiterals
	s.mu.RUnlock()

	type kv struct {
		name, value string
	}
	kvs := make([]kv, 0, len(literals))
	for k, v := range literals {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].name < kvs[j].name })

	literalValues := make([]string, len(kvs))
	literalNames := make([]string, len(kvs))
	for i, pair := range kvs {
		literalNames[i] = pair.name
		literalValues[i] = pair.value
	}

	if automaton == nil || cachedVersion != version || !sliceEqual(cachedLiterals, literalValues) {
		automaton = NewAhoCorasick(literalValues)
		s.mu.Lock()
		s.cachedAutomaton = automaton
		s.cachedVersion = version
		s.cachedLiterals = literalValues
		s.mu.Unlock()
	}

	acMatches := automaton.Search(text)
	var matches []LeakMatch
	for _, m := range acMatches {
		matches = append(matches, LeakMatch{
			PatternName: fmt.Sprintf("resolved_secret:%s", literalNames[m.PatternIndex]),
			Value:       redactValue(literalValues[m.PatternIndex]),
			Position:    m.Position,
			Length:      len(literalValues[m.PatternIndex]),
			Severity:    SeverityCritical,
		})
	}
	return matches
}

type replacement struct {
	start int
	end   int
	name  string
}

func (s *LeakScanner) Redact(text string) (string, []LeakMatch) {
	matches := s.Scan(text)
	if len(matches) == 0 {
		return text, nil
	}

	var repls []replacement
	for _, m := range matches {
		repls = append(repls, replacement{
			start: m.Position,
			end:   m.Position + m.Length,
			name:  m.PatternName,
		})
	}

	sortReplacements(repls)
	repls = mergeOverlapping(repls)

	result := make([]byte, 0, len(text))
	prev := 0
	for _, r := range repls {
		if r.start < prev {
			continue
		}
		result = append(result, text[prev:r.start]...)
		result = append(result, fmt.Sprintf("[REDACTED:%s]", r.name)...)
		prev = r.end
	}
	result = append(result, text[prev:]...)

	return string(result), matches
}

func sortReplacements(repls []replacement) {
	for i := 1; i < len(repls); i++ {
		for j := i; j > 0 && repls[j].start < repls[j-1].start; j-- {
			repls[j], repls[j-1] = repls[j-1], repls[j]
		}
	}
}

func mergeOverlapping(repls []replacement) []replacement {
	if len(repls) == 0 {
		return nil
	}
	merged := []replacement{repls[0]}
	for i := 1; i < len(repls); i++ {
		last := &merged[len(merged)-1]
		if repls[i].start <= last.end {
			if repls[i].end > last.end {
				last.end = repls[i].end
			}
		} else {
			merged = append(merged, repls[i])
		}
	}
	return merged
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
