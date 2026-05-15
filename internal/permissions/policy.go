// Package permissions provides a declarative permission policy system
// for controlling tool access at project and user level.
package permissions

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Colin4k1024/hermesx/internal/config"
	"gopkg.in/yaml.v3"
)

// Action defines what happens when a rule matches.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask" // falls through to existing ApprovalHandler
)

// PolicyRule defines a single permission rule.
type PolicyRule struct {
	Tool     string   `yaml:"tool"`     // tool name or "*" for all
	Action   Action   `yaml:"action"`   // "allow", "deny", "ask"
	Paths    []string `yaml:"paths"`    // glob patterns for file operations (optional)
	Commands []string `yaml:"commands"` // glob patterns for terminal commands (optional)
	Reason   string   `yaml:"reason"`   // why this rule exists
}

// PermissionPolicy contains the full policy configuration.
type PermissionPolicy struct {
	Rules   []PolicyRule `yaml:"rules"`
	Default Action       `yaml:"default"` // fallback when no rule matches (default: "ask")
}

// PolicyDecision is the result of evaluating a tool call against the policy.
type PolicyDecision struct {
	Action Action
	Reason string // explanation for the decision
}

// policyState holds the loaded policy (thread-safe singleton).
var (
	policyOnce sync.Once
	policyMu   sync.RWMutex
	loaded     *PermissionPolicy
)

// LoadPolicy loads the permission policy from project and user config directories.
// Project-level rules take precedence over user-level rules.
// Resolution: project/.hermes/permissions.yaml > ~/.hermes/permissions.yaml
func LoadPolicy() *PermissionPolicy {
	policyOnce.Do(func() {
		loaded = loadPolicyFromDisk()
	})
	policyMu.RLock()
	defer policyMu.RUnlock()
	return loaded
}

// ReloadPolicy forces a policy reload (useful after config changes).
func ReloadPolicy() *PermissionPolicy {
	policyMu.Lock()
	defer policyMu.Unlock()
	loaded = loadPolicyFromDisk()
	policyOnce = sync.Once{}
	return loaded
}

// SetPolicy replaces the current policy (used in tests).
func SetPolicy(p *PermissionPolicy) {
	policyMu.Lock()
	defer policyMu.Unlock()
	loaded = p
	policyOnce = sync.Once{}
	// Mark as already loaded so LoadPolicy() returns our value.
	policyOnce.Do(func() {})
}

func loadPolicyFromDisk() *PermissionPolicy {
	policy := &PermissionPolicy{
		Default: ActionAsk, // default to "ask" for backward compat with ApprovalHandler
	}

	// Layer 1: User-level policy (~/.hermes/permissions.yaml)
	userPath := filepath.Join(config.HermesHome(), "permissions.yaml")
	if userPolicy := readPolicyFile(userPath); userPolicy != nil {
		policy = mergePolicies(policy, userPolicy)
	}

	// Layer 2: Project-level policy ({project}/.hermes/permissions.yaml)
	cwd, err := os.Getwd()
	if err == nil {
		root := config.FindProjectRoot(cwd)
		if root != "" {
			projectPath := filepath.Join(root, config.ProjectConfigDir, "permissions.yaml")
			if projectPolicy := readPolicyFile(projectPath); projectPolicy != nil {
				policy = mergePolicies(policy, projectPolicy)
			}
		}
	}

	if len(policy.Rules) > 0 {
		slog.Debug("Permission policy loaded", "rules", len(policy.Rules))
	}
	return policy
}

func readPolicyFile(path string) *PermissionPolicy {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var policy PermissionPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		slog.Warn("Failed to parse permissions file", "path", path, "error", err)
		return nil
	}
	return &policy
}

// mergePolicies merges src rules onto dst. Source rules are appended
// (higher priority evaluated first during Check).
func mergePolicies(dst, src *PermissionPolicy) *PermissionPolicy {
	result := &PermissionPolicy{
		Default: dst.Default,
		Rules:   make([]PolicyRule, 0, len(dst.Rules)+len(src.Rules)),
	}
	// Source rules come first (higher priority).
	result.Rules = append(result.Rules, src.Rules...)
	result.Rules = append(result.Rules, dst.Rules...)

	if src.Default != "" {
		result.Default = src.Default
	}
	return result
}

// Check evaluates whether a tool call is permitted according to the policy.
// Parameters:
//   - toolName: the name of the tool being invoked
//   - command: the command string (for terminal tools), empty for non-terminal tools
//   - filePath: the file path (for file operation tools), empty for non-file tools
func Check(toolName, command, filePath string) PolicyDecision {
	policy := LoadPolicy()
	if policy == nil || len(policy.Rules) == 0 {
		return PolicyDecision{Action: ActionAsk, Reason: "no policy loaded"}
	}

	for _, rule := range policy.Rules {
		if !matchesTool(rule.Tool, toolName) {
			continue
		}

		// If rule has path restrictions, check them.
		if len(rule.Paths) > 0 && filePath != "" {
			if !matchesGlobs(rule.Paths, filePath) {
				continue
			}
		}

		// If rule has command restrictions, check them.
		if len(rule.Commands) > 0 && command != "" {
			if !matchesGlobs(rule.Commands, command) {
				continue
			}
		}

		// Rule matches.
		return PolicyDecision{
			Action: rule.Action,
			Reason: rule.Reason,
		}
	}

	// No rule matched; use default.
	return PolicyDecision{
		Action: policy.Default,
		Reason: "default policy",
	}
}

// matchesTool checks if a rule's tool pattern matches the given tool name.
func matchesTool(pattern, toolName string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	return strings.EqualFold(pattern, toolName)
}

// matchesGlobs checks if any glob pattern matches the given value.
func matchesGlobs(patterns []string, value string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, value)
		if err == nil && matched {
			return true
		}
		// Also check if the pattern is a simple substring match.
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}
