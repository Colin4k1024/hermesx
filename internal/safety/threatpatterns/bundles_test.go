package threatpatterns
package threatpatterns_test

import (
	"testing"

	"github.com/Colin4k1024/hermesx/internal/safety/threatpatterns"
)

func TestBundlesHaveNameAndVersion(t *testing.T) {
	for _, b := range threatpatterns.All() {
		if b.Name == "" {
			t.Errorf("bundle has empty name")
		}
		if b.Version == "" {
			t.Errorf("bundle %q has empty version", b.Name)
		}
	}
}

func TestBundlesPatternsNotEmpty(t *testing.T) {
	for _, b := range threatpatterns.All() {
		if len(b.Patterns) == 0 {
			t.Errorf("bundle %q has no patterns", b.Name)
		}
		for _, p := range b.Patterns {
			if p.Name == "" {
				t.Errorf("bundle %q: pattern has empty name", b.Name)
			}
			if p.Regex == "" {
				t.Errorf("bundle %q: pattern %q has empty regex", b.Name, p.Name)
			}
			if p.Severity <= 0 {
				t.Errorf("bundle %q: pattern %q has non-positive severity %d", b.Name, p.Name, p.Severity)
			}
		}
	}
}

func TestBundleNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range threatpatterns.All() {
		if seen[b.Name] {
			t.Errorf("duplicate bundle name %q", b.Name)
		}
		seen[b.Name] = true
	}
}

func TestAllBundlesCount(t *testing.T) {
	all := threatpatterns.All()
	const want = 7
	if len(all) != want {
		t.Errorf("All() returned %d bundles, want %d", len(all), want)
	}
}

func TestIndividualBundleFunctions(t *testing.T) {
	cases := []struct {
		name string
		fn   func() threatpatterns.Bundle
	}{
		{"PromptInjection", threatpatterns.PromptInjection},
		{"RoleHijack", threatpatterns.RoleHijack},
		{"PromptExtraction", threatpatterns.PromptExtraction},
		{"DelimiterInjection", threatpatterns.DelimiterInjection},
		{"EncodingAttack", threatpatterns.EncodingAttack},
		{"SafetyBypass", threatpatterns.SafetyBypass},
		{"IndirectInjection", threatpatterns.IndirectInjection},
	}
	for _, tc := range cases {
		b := tc.fn()
		if b.Name == "" {
			t.Errorf("%s(): Name is empty", tc.name)
		}
		if len(b.Patterns) == 0 {
			t.Errorf("%s(): no patterns", tc.name)
		}
	}
}
