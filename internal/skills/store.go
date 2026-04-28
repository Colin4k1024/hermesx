package skills

import "context"

// SkillLoader abstracts where skills are loaded from.
// Implementations: LocalSkillLoader (filesystem), MinIOSkillLoader (S3).
type SkillLoader interface {
	LoadAll(ctx context.Context) ([]*SkillEntry, error)
	Find(ctx context.Context, name string) (*SkillEntry, error)
}
