package evolution

import (
	"errors"
	"time"
)

var ErrPolicyVersionNotFound = errors.New("evolution: policy version not found")

const (
	// SharingDisabled keeps all genes tenant-local.
	SharingDisabled = "disabled"
	// SharingAnonymous allows tenant genes to be copied into a shared namespace
	// without source tenant attribution for normal replay.
	SharingAnonymous = "anonymous"
	// SharingTrusted allows shared replay while preserving ContributorID for
	// governance and review workflows.
	SharingTrusted = "trusted"
)

const sharedTaskClassPrefix = "__shared__:"

func sharedTaskClass(taskClass string) string {
	return sharedTaskClassPrefix + taskClass
}

func normalizeSharingMode(mode string) string {
	switch mode {
	case SharingAnonymous, SharingTrusted:
		return mode
	default:
		return SharingDisabled
	}
}

// SharingPolicySnapshot is returned by admin governance endpoints.
type SharingPolicySnapshot struct {
	Mode         string   `json:"mode"`
	SharedPrefix string   `json:"shared_prefix"`
	Levels       []string `json:"levels"`
	Version      int64    `json:"version"`
}

// TenantSharingPolicy controls one tenant's participation in shared learning.
type TenantSharingPolicy struct {
	TenantID         string   `json:"tenant_id" yaml:"tenant_id"`
	ConsumeShared    bool     `json:"consume_shared" yaml:"consume_shared"`
	ContributionMode string   `json:"contribution_mode" yaml:"contribution_mode"`
	Labels           []string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// EffectiveTenantSharingPolicy is the resolved policy after global controls are applied.
type EffectiveTenantSharingPolicy struct {
	TenantID                  string   `json:"tenant_id"`
	GlobalMode                string   `json:"global_mode"`
	ConsumeShared             bool     `json:"consume_shared"`
	ContributionMode          string   `json:"contribution_mode"`
	EffectiveContributionMode string   `json:"effective_contribution_mode"`
	Labels                    []string `json:"labels,omitempty"`
	Version                   int64    `json:"version"`
}

// SharingPolicyHistoryEntry is an auditable snapshot of one policy version.
type SharingPolicyHistoryEntry struct {
	ScopeType        string     `json:"scope_type"`
	ScopeID          string     `json:"scope_id"`
	Version          int64      `json:"version"`
	Reason           string     `json:"reason"`
	ChangedAt        time.Time  `json:"changed_at"`
	Mode             string     `json:"mode,omitempty"`
	ConsumeShared    *bool      `json:"consume_shared,omitempty"`
	ContributionMode string     `json:"contribution_mode,omitempty"`
	Labels           []string   `json:"labels,omitempty"`
}
