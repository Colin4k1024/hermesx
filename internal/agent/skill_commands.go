package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/skills"
)

// InjectSkillAsUserMessage loads a skill by name and returns its content
// formatted as a user-role message. This approach avoids modifying the system
// prompt, which would invalidate the Anthropic prompt cache.
//
// Returns the skill body wrapped in an instruction block, or an error if
// the skill was not found or could not be loaded.
func InjectSkillAsUserMessage(skillName string) (string, error) {
	skillName = strings.TrimPrefix(skillName, "/")

	entry, err := skills.FindSkill(skillName)
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", skillName, err)
	}

	// Build the injection message.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Skill activated: %s", entry.Meta.Name))
	if entry.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", entry.Meta.Description))
	}
	sb.WriteString("]\n\n")
	sb.WriteString(entry.Body)

	return sb.String(), nil
}

// GetAvailableSkillCommands returns slash-command strings (e.g. "/summarize")
// for all installed skills that are compatible with the current platform.
func GetAvailableSkillCommands() []string {
	allSkills, err := skills.LoadAllSkills()
	if err != nil {
		return nil
	}

	commands := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		commands = append(commands, "/"+s.Meta.Name)
	}
	return commands
}

// InjectSkill loads a skill via the agent's SkillLoader (if set) and returns
// its content formatted as a user-role message. Falls back to the global
// InjectSkillAsUserMessage when no SkillLoader is configured.
func (a *AIAgent) InjectSkill(skillName string) (string, error) {
	skillName = strings.TrimPrefix(skillName, "/")

	if a.skillLoader == nil {
		return InjectSkillAsUserMessage(skillName)
	}

	entry, err := a.skillLoader.Find(context.Background(), skillName)
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", skillName, err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Skill activated: %s", entry.Meta.Name))
	if entry.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", entry.Meta.Description))
	}
	sb.WriteString("]\n\n")
	sb.WriteString(entry.Body)
	return sb.String(), nil
}

// IsSkill checks whether the input matches a skill available through the
// agent's SkillLoader. Falls back to global IsSkillCommand when no loader is set.
func (a *AIAgent) IsSkill(input string) bool {
	if a.skillLoader == nil {
		return IsSkillCommand(input)
	}

	if !strings.HasPrefix(input, "/") {
		return false
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	name := strings.TrimPrefix(parts[0], "/")
	if name == "" {
		return false
	}
	_, err := a.skillLoader.Find(context.Background(), name)
	return err == nil
}

// AvailableSkillCommands returns slash-command strings for skills available
// through the agent's SkillLoader. Falls back to global GetAvailableSkillCommands.
func (a *AIAgent) AvailableSkillCommands() []string {
	if a.skillLoader == nil {
		return GetAvailableSkillCommands()
	}

	allSkills, err := a.skillLoader.LoadAll(context.Background())
	if err != nil {
		return nil
	}
	commands := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		commands = append(commands, "/"+s.Meta.Name)
	}
	return commands
}

// IsSkillCommand checks whether the given input looks like a skill slash command.
// It verifies the input starts with "/" and that the name matches an installed skill.
func IsSkillCommand(input string) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}

	// Extract the command name (first word after /).
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	name := strings.TrimPrefix(parts[0], "/")
	if name == "" {
		return false
	}

	_, err := skills.FindSkill(name)
	return err == nil
}
