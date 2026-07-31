package permissions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/config"
)

func TestCheck_DenyRule(t *testing.T) {
	SetPolicy(&PermissionPolicy{
		Default: ActionAsk,
		Rules: []PolicyRule{
			{Tool: "terminal", Action: ActionDeny, Commands: []string{"rm -rf"}, Reason: "dangerous deletion"},
			{Tool: "file_write", Action: ActionDeny, Paths: []string{"/etc/*"}, Reason: "system files protected"},
			{Tool: "web_search", Action: ActionAllow, Reason: "always allowed"},
		},
	})

	tests := []struct {
		name     string
		tool     string
		command  string
		path     string
		expected Action
	}{
		{"deny terminal rm -rf", "terminal", "rm -rf /tmp/foo", "", ActionDeny},
		{"allow terminal safe cmd", "terminal", "ls -la", "", ActionAsk}, // no matching rule, falls to default
		{"deny file_write to /etc", "file_write", "", "/etc/passwd", ActionDeny},
		{"allow file_write to /tmp", "file_write", "", "/tmp/data.txt", ActionAsk}, // no match
		{"allow web_search always", "web_search", "", "", ActionAllow},
		{"default for unknown tool", "unknown_tool", "", "", ActionAsk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Check(tt.tool, tt.command, tt.path)
			if decision.Action != tt.expected {
				t.Errorf("Check(%q, %q, %q) = %q, want %q", tt.tool, tt.command, tt.path, decision.Action, tt.expected)
			}
		})
	}
}

func TestCheck_WildcardTool(t *testing.T) {
	SetPolicy(&PermissionPolicy{
		Default: ActionAllow,
		Rules: []PolicyRule{
			{Tool: "*", Action: ActionDeny, Commands: []string{"sudo"}, Reason: "no sudo"},
		},
	})

	decision := Check("terminal", "sudo rm -rf /", "")
	if decision.Action != ActionDeny {
		t.Errorf("expected deny for sudo command, got %q", decision.Action)
	}

	decision = Check("terminal", "echo hello", "")
	if decision.Action != ActionAllow {
		t.Errorf("expected allow for safe command (default), got %q", decision.Action)
	}
}

func TestLoadPolicyFromFile(t *testing.T) {
	// Create temp project with permissions.yaml
	tmpDir := t.TempDir()
	hermesDir := filepath.Join(tmpDir, ".hermes")
	os.MkdirAll(hermesDir, 0755)
	os.WriteFile(filepath.Join(hermesDir, "permissions.yaml"), []byte(`
default: deny
rules:
  - tool: web_search
    action: allow
    reason: research is always ok
  - tool: terminal
    action: ask
    commands: ["docker *"]
    reason: container ops need confirmation
`), 0644)

	policy := readPolicyFile(filepath.Join(hermesDir, "permissions.yaml"))
	if policy == nil {
		t.Fatal("readPolicyFile returned nil")
	}
	if policy.Default != ActionDeny {
		t.Errorf("default = %q, want 'deny'", policy.Default)
	}
	if len(policy.Rules) != 2 {
		t.Errorf("rules count = %d, want 2", len(policy.Rules))
	}
}

func TestMatchesGlobs_NoSubstringFallback(t *testing.T) {
	// Verify that glob matching does NOT fall back to substring matching.
	// This is a security fix: pattern "*.env" must not match "/etc/environment".
	tests := []struct {
		name     string
		patterns []string
		value    string
		want     bool
	}{
		{"exact glob match", []string{"*.env"}, "secret.env", true},
		{"glob no match different ext", []string{"*.env"}, "secret.txt", false},
		{"no substring bypass for *.env", []string{"*.env"}, "/etc/environment", false},
		{"no substring bypass for secret", []string{"secret"}, "my-secret-file.txt", false},
		{"wildcard matches all", []string{"*"}, "anything", true},
		{"full path glob", []string{"/etc/*"}, "/etc/passwd", true},
		{"full path glob no match", []string{"/etc/*"}, "/tmp/file", false},
		{"multiple patterns first match", []string{"*.env", "*.key"}, "test.key", true},
		{"multiple patterns no match", []string{"*.env", "*.key"}, "test.txt", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesGlobs(tt.patterns, tt.value)
			if got != tt.want {
				t.Errorf("matchesGlobs(%v, %q) = %v, want %v", tt.patterns, tt.value, got, tt.want)
			}
		})
	}
}

func TestCheck_DenyRule_SubstringBypass(t *testing.T) {
	// Verify that a deny rule for "*.env" does NOT block access to
	// files that merely contain "env" in their path.
	SetPolicy(&PermissionPolicy{
		Default: ActionAllow,
		Rules: []PolicyRule{
			{Tool: "file_read", Action: ActionDeny, Paths: []string{"*.env"}, Reason: "env files protected"},
		},
	})

	tests := []struct {
		name     string
		path     string
		expected Action
	}{
		{"deny .env file", "secrets.env", ActionDeny},
		{"allow .txt file", "data.txt", ActionAllow},
		{"allow environment file", "/etc/environment", ActionAllow},
		{"allow env directory", "env/config.yaml", ActionAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Check("file_read", "", tt.path)
			if decision.Action != tt.expected {
				t.Errorf("Check(file_read, %q) = %q, want %q", tt.path, decision.Action, tt.expected)
			}
		})
	}
}

func TestMergePolicies(t *testing.T) {
	base := &PermissionPolicy{
		Default: ActionAsk,
		Rules: []PolicyRule{
			{Tool: "terminal", Action: ActionAsk, Reason: "base rule"},
		},
	}
	override := &PermissionPolicy{
		Default: ActionDeny,
		Rules: []PolicyRule{
			{Tool: "terminal", Action: ActionDeny, Reason: "project override"},
		},
	}

	merged := mergePolicies(base, override)
	if merged.Default != ActionDeny {
		t.Errorf("merged default = %q, want 'deny'", merged.Default)
	}
	// Override rules come first (higher priority).
	if len(merged.Rules) != 2 {
		t.Fatalf("merged rules count = %d, want 2", len(merged.Rules))
	}
	if merged.Rules[0].Reason != "project override" {
		t.Errorf("first rule should be override, got %q", merged.Rules[0].Reason)
	}
}

func TestFindProjectRoot_Integration(t *testing.T) {
	// Verify FindProjectRoot is accessible from this package via config.
	tmpDir := t.TempDir()
	hermesDir := filepath.Join(tmpDir, ".hermes")
	os.MkdirAll(hermesDir, 0755)
	os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte("model: test\n"), 0644)

	root := config.FindProjectRoot(tmpDir)
	if root != tmpDir {
		t.Errorf("FindProjectRoot = %q, want %q", root, tmpDir)
	}
}
