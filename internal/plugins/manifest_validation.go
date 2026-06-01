package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginManifestIssue describes a bundled plugin packaging or metadata problem.
type PluginManifestIssue struct {
	Path    string
	Message string
}

// ValidateBundledPluginManifests checks a bundled plugin tree for missing or
// invalid plugin.yaml/plugin.yml files. Category directories are allowed when
// they contain nested plugin manifests.
func ValidateBundledPluginManifests(root string) ([]PluginManifestIssue, error) {
	if root == "" {
		return nil, fmt.Errorf("plugin root is required")
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat plugin root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugin root is not a directory: %s", root)
	}

	dirs := map[string]*pluginDirInfo{}
	ensureDir := func(path string) *pluginDirInfo {
		if dirs[path] == nil {
			dirs[path] = &pluginDirInfo{path: path}
		}
		return dirs[path]
	}
	ensureDir(root)

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			ensureDir(path)
			return nil
		}

		dir := ensureDir(filepath.Dir(path))
		name := d.Name()
		switch name {
		case "plugin.yaml", "plugin.yml":
			dir.hasManifest = true
			return validatePluginManifestFile(path, dir)
		default:
			if isPluginImplementationFile(name) {
				dir.hasImplementation = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk plugin root: %w", err)
	}

	for path, dir := range dirs {
		if path == root || !dir.hasManifest {
			continue
		}
		for parent := filepath.Dir(path); parent != "." && parent != path; parent = filepath.Dir(parent) {
			if info := dirs[parent]; info != nil {
				info.hasDescendantManifest = true
			}
			if parent == root {
				break
			}
		}
	}

	var issues []PluginManifestIssue
	for path, dir := range dirs {
		if path == root {
			continue
		}
		issues = append(issues, dir.issues...)
		if !dir.hasManifest && !dir.hasDescendantManifest && dir.hasImplementation {
			issues = append(issues, PluginManifestIssue{
				Path:    path,
				Message: "plugin implementation directory is missing plugin.yaml/plugin.yml",
			})
		}
	}
	return issues, nil
}

type pluginDirInfo struct {
	path                  string
	hasManifest           bool
	hasDescendantManifest bool
	hasImplementation     bool
	issues                []PluginManifestIssue
}

func validatePluginManifestFile(path string, dir *pluginDirInfo) error {
	data, err := os.ReadFile(path)
	if err != nil {
		dir.issues = append(dir.issues, PluginManifestIssue{Path: path, Message: err.Error()})
		return nil
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		dir.issues = append(dir.issues, PluginManifestIssue{
			Path:    path,
			Message: "invalid plugin manifest: " + err.Error(),
		})
		return nil
	}
	if strings.TrimSpace(manifest.Name) == "" {
		dir.issues = append(dir.issues, PluginManifestIssue{
			Path:    path,
			Message: "plugin manifest missing required name",
		})
	}
	return nil
}

func isPluginImplementationFile(name string) bool {
	switch name {
	case "__init__.py", "provider.py", "adapter.py", "tools.py", "plugin.py",
		"provider.go", "adapter.go", "tools.go", "plugin.go":
		return true
	default:
		return false
	}
}
