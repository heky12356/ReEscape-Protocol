package character

import (
	"fmt"
	"strings"
)

func NormalizeConfig(cfg *CharacterConfig) {
	if cfg == nil {
		return
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Description = strings.TrimSpace(cfg.Description)

	cfg.Identity.RoleName = strings.TrimSpace(cfg.Identity.RoleName)
	if cfg.Identity.RoleName == "" {
		cfg.Identity.RoleName = cfg.Name
	}
	cfg.Identity.ProductIdentity = strings.TrimSpace(cfg.Identity.ProductIdentity)
	cfg.Identity.SelfReference = strings.TrimSpace(cfg.Identity.SelfReference)
	cfg.Identity.IdentityPolicy = strings.TrimSpace(cfg.Identity.IdentityPolicy)

	cfg.Background.Age = strings.TrimSpace(cfg.Background.Age)
	cfg.Background.Occupation = strings.TrimSpace(cfg.Background.Occupation)
	cfg.Background.Traits = normalizeStringSlice(cfg.Background.Traits)
	cfg.Background.Interests = normalizeStringSlice(cfg.Background.Interests)
	cfg.Background.Habits = normalizeStringSlice(cfg.Background.Habits)
	cfg.Background.Skills = normalizeStringSlice(cfg.Background.Skills)

	cfg.Voice.Tone = strings.TrimSpace(cfg.Voice.Tone)
	cfg.Voice.Style = strings.TrimSpace(cfg.Voice.Style)
	cfg.Voice.Pacing = strings.TrimSpace(cfg.Voice.Pacing)
	cfg.Voice.Vocabulary = normalizeStringSlice(cfg.Voice.Vocabulary)
	cfg.Voice.Avoid = normalizeStringSlice(cfg.Voice.Avoid)

	cfg.Boundaries.DoNotReveal = normalizeStringSlice(cfg.Boundaries.DoNotReveal)
	cfg.Boundaries.SafetyBoundaries = normalizeStringSlice(cfg.Boundaries.SafetyBoundaries)
	cfg.Boundaries.RelationshipRules = normalizeStringSlice(cfg.Boundaries.RelationshipRules)

	cfg.Relationship.DefaultStage = strings.TrimSpace(cfg.Relationship.DefaultStage)
	cfg.Relationship.Addressing = strings.TrimSpace(cfg.Relationship.Addressing)
	cfg.Relationship.IntimacyRule = strings.TrimSpace(cfg.Relationship.IntimacyRule)

	for i := range cfg.Examples {
		cfg.Examples[i].Situation = strings.TrimSpace(cfg.Examples[i].Situation)
		cfg.Examples[i].User = strings.TrimSpace(cfg.Examples[i].User)
		cfg.Examples[i].Reply = strings.TrimSpace(cfg.Examples[i].Reply)
	}
	cfg.Examples = filterExamples(cfg.Examples)
}

func ValidateConfig(cfg *CharacterConfig) error {
	NormalizeConfig(cfg)
	if cfg == nil || cfg.Name == "" {
		return fmt.Errorf("character config.name is required")
	}
	for i, example := range cfg.Examples {
		if example.User == "" || example.Reply == "" {
			return fmt.Errorf("examples[%d] must include user and reply", i)
		}
	}
	return nil
}

func normalizeStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func filterExamples(items []CharacterExample) []CharacterExample {
	if items == nil {
		return []CharacterExample{}
	}
	result := make([]CharacterExample, 0, len(items))
	for _, item := range items {
		if item.Situation == "" && item.User == "" && item.Reply == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}
