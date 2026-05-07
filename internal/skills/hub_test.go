package skills

import (
	"testing"
)

func TestDefaultSources(t *testing.T) {
	sources := DefaultSources()
	if len(sources) == 0 {
		t.Error("Expected non-empty default sources")
	}

	// Check expected sources exist
	hasAgentskills := false
	hasHermesOfficial := false
	for _, s := range sources {
		if s.Name == "agentskills.io" {
			hasAgentskills = true
			if s.Type != "url" {
				t.Errorf("Expected agentskills.io type 'url', got '%s'", s.Type)
			}
			if s.Trust != "community" {
				t.Errorf("Expected agentskills.io trust 'community', got '%s'", s.Trust)
			}
		}
		if s.Name == "hermesx-official" {
			hasHermesOfficial = true
			if s.Type != "github" {
				t.Errorf("Expected hermes-official type 'github', got '%s'", s.Type)
			}
			if s.Trust != "trusted" {
				t.Errorf("Expected hermes-official trust 'trusted', got '%s'", s.Trust)
			}
		}
	}

	if !hasAgentskills {
		t.Error("Expected agentskills.io source")
	}
	if !hasHermesOfficial {
		t.Error("Expected hermes-official source")
	}
}

func TestHubSource_Fields(t *testing.T) {
	s := HubSource{
		Name:  "test-source",
		Type:  "url",
		URL:   "https://example.com/api/skills",
		Trust: "community",
	}

	if s.Name != "test-source" {
		t.Errorf("Expected 'test-source', got '%s'", s.Name)
	}
	if s.Type != "url" {
		t.Errorf("Expected 'url', got '%s'", s.Type)
	}
}

func TestHubSkill_Fields(t *testing.T) {
	s := HubSkill{
		Name:        "test-skill",
		Description: "A test skill",
		Version:     "1.0.0",
		Author:      "Test",
		Tags:        []string{"test", "unit"},
		Source:      "test-source",
		URL:         "https://example.com/skill",
		Trust:       "trusted",
	}

	if s.Name != "test-skill" {
		t.Errorf("Expected 'test-skill', got '%s'", s.Name)
	}
	if len(s.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(s.Tags))
	}
}

// SearchHub does network calls which we can't unit test reliably,
// but we can test that it doesn't panic with an empty query.
// Note: This test may fail if no network is available, so we only
// check it doesn't panic.
func TestSearchHub_EmptyQuery_NoPanic(t *testing.T) {
	// This makes real network calls, so just check it doesn't crash
	// and returns without error (the results may be empty if offline).
	results, err := SearchHub("")
	if err != nil {
		// Errors from network are acceptable in tests
		t.Skipf("SearchHub failed (likely offline): %v", err)
	}
	// Results may be empty or non-empty depending on connectivity
	_ = results
}
