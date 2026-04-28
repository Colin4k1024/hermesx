package skills

import "context"

// LocalSkillLoader loads skills from the local filesystem (~/.hermes/skills/).
type LocalSkillLoader struct{}

func NewLocalSkillLoader() *LocalSkillLoader {
	return &LocalSkillLoader{}
}

func (l *LocalSkillLoader) LoadAll(_ context.Context) ([]*SkillEntry, error) {
	return LoadAllSkills()
}

func (l *LocalSkillLoader) Find(_ context.Context, name string) (*SkillEntry, error) {
	return FindSkill(name)
}
