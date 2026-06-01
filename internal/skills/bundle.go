package skills

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BundleManifest describes a curated collection of skill references.
// Bundles do not embed skill content; they reference canonical skills by name.
// A bundle.yaml placed at the root of a bundle directory defines the manifest.
//
// Example layout:
//
//	my-bundle/
//	  bundle.yaml
//	  code-review/SKILL.md    (referenced by name "code-review")
//	  testing/SKILL.md        (referenced by name "testing")
type BundleManifest struct {
	Name        string           `yaml:"name" json:"name"`
	Version     string           `yaml:"version" json:"version"`
	Description string           `yaml:"description" json:"description"`
	Author      string           `yaml:"author" json:"author"`
	MinVersion  string           `yaml:"min_version" json:"min_version,omitempty"`
	Skills      []BundleSkillRef `yaml:"skills" json:"skills"`
	// Policy governs tenant override behaviour.
	Policy BundlePolicy `yaml:"policy" json:"policy"`
}

// BundleSkillRef is a reference to a skill within a bundle.
type BundleSkillRef struct {
	// Name is the skill directory name as it appears under the bundle root.
	Name string `yaml:"name" json:"name"`
	// Required, when true, causes ValidateBundleManifest to report an issue
	// if the skill directory is missing from the bundle root.
	Required bool `yaml:"required" json:"required"`
	// MinVersion is an optional minimum version constraint for the referenced skill.
	MinVersion string `yaml:"min_version" json:"min_version,omitempty"`
}

// BundlePolicy governs how bundle skills interact with tenant overrides.
type BundlePolicy struct {
	// AllowTenantOverride, when true, permits tenants to shadow or replace
	// individual skills that originate from this bundle.
	AllowTenantOverride bool `yaml:"allow_tenant_override" json:"allow_tenant_override"`
	// AllowUserModification, when true, permits individual users within a
	// tenant to shadow bundle-provided skills.
	AllowUserModification bool `yaml:"allow_user_modification" json:"allow_user_modification"`
}

// BundleManifestIssue represents a single validation finding.
type BundleManifestIssue struct {
	Kind    BundleIssueKind
	Message string
	// Ref is the skill name related to the issue, if applicable.
	Ref string
}

func (i BundleManifestIssue) Error() string { return i.Message }

// BundleIssueKind categorises the type of a BundleManifestIssue.
type BundleIssueKind string

const (
	// BundleIssueNoBundleFile is reported when no bundle.yaml exists at root.
	BundleIssueNoBundleFile BundleIssueKind = "no_bundle_file"
	// BundleIssueInvalidYAML is reported when bundle.yaml cannot be parsed.
	BundleIssueInvalidYAML BundleIssueKind = "invalid_yaml"
	// BundleIssueMissingName is reported when the bundle manifest has no name.
	BundleIssueMissingName BundleIssueKind = "missing_name"
	// BundleIssueMissingRequiredSkill is reported when a required skill ref has
	// no matching directory at the bundle root.
	BundleIssueMissingRequiredSkill BundleIssueKind = "missing_required_skill"
)

// LoadBundleManifest reads and parses a bundle.yaml from the given path.
func LoadBundleManifest(path string) (*BundleManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle manifest: %w", err)
	}
	var m BundleManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse bundle manifest: %w", err)
	}
	return &m, nil
}

// ValidateBundleManifest checks the bundle.yaml at root and verifies that all
// required skill references have matching directories.
//
// A missing bundle root is treated as a no-op and returns nil issues, so
// deployments that do not ship bundles remain compatible.
func ValidateBundleManifest(root string) []BundleManifestIssue {
	if root == "" {
		return nil
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	manifestPath := filepath.Join(root, "bundle.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return []BundleManifestIssue{
			{Kind: BundleIssueNoBundleFile, Message: fmt.Sprintf("bundle root %q has no bundle.yaml", root)},
		}
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return []BundleManifestIssue{
			{Kind: BundleIssueInvalidYAML, Message: fmt.Sprintf("read bundle.yaml: %v", err)},
		}
	}

	var m BundleManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return []BundleManifestIssue{
			{Kind: BundleIssueInvalidYAML, Message: fmt.Sprintf("parse bundle.yaml: %v", err)},
		}
	}

	var issues []BundleManifestIssue

	if m.Name == "" {
		issues = append(issues, BundleManifestIssue{
			Kind:    BundleIssueMissingName,
			Message: "bundle.yaml is missing required field: name",
		})
	}

	for _, ref := range m.Skills {
		if !ref.Required {
			continue
		}
		skillDir := filepath.Join(root, ref.Name)
		if _, err := os.Stat(skillDir); os.IsNotExist(err) {
			issues = append(issues, BundleManifestIssue{
				Kind:    BundleIssueMissingRequiredSkill,
				Ref:     ref.Name,
				Message: fmt.Sprintf("required skill %q is missing from bundle root %q", ref.Name, root),
			})
		}
	}

	return issues
}
