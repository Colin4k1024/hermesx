package skills

import (
	"context"
)

// FilteredSkillLoader wraps a SkillLoader and only returns skills whose names
// are in the allowed set. This is used to restrict agent profiles to a subset
// of available skills.
type FilteredSkillLoader struct {
	inner   SkillLoader
	allowed map[string]bool
}

// NewFilteredSkillLoader creates a loader that filters skills by name.
func NewFilteredSkillLoader(inner SkillLoader, allowedNames []string) *FilteredSkillLoader {
	allowed := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowed[name] = true
	}
	return &FilteredSkillLoader{inner: inner, allowed: allowed}
}

func (f *FilteredSkillLoader) LoadAll(ctx context.Context) ([]*SkillEntry, error) {
	all, err := f.inner.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []*SkillEntry
	for _, entry := range all {
		if f.allowed[entry.Meta.Name] || f.allowed[entry.DirName] {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func (f *FilteredSkillLoader) Find(ctx context.Context, name string) (*SkillEntry, error) {
	if !f.allowed[name] {
		return nil, nil
	}
	return f.inner.Find(ctx, name)
}
