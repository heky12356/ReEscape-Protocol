package config

import (
	"strings"

	"project-yume/internal/character"
)

type PromptSections struct {
	BasePrompt      string `json:"basePrompt"`
	UserPrompt      string `json:"userPrompt"`
	CharacterPrompt string `json:"characterPrompt"`
	EffectivePrompt string `json:"effectivePrompt"`
}

func CurrentPromptSections() PromptSections {
	return PromptSections{
		BasePrompt:      config.BasePrompt,
		UserPrompt:      config.UserPrompt,
		CharacterPrompt: config.CharacterPrompt,
		EffectivePrompt: config.AiPrompt,
	}
}

func GetCharacterToneSummary() string {
	if cm == nil || cm.GetConfig() == nil {
		return ""
	}
	return character.BuildToneSummary(*cm.GetConfig())
}

func applyPromptSections(basePrompt, userPrompt string) {
	characterPrompt := ""
	if cm != nil {
		characterPrompt = cm.GetPrompt()
	}
	characterPrompt = joinPromptSections(characterPrompt, buildCharacterIdentityRuntimePolicy())

	config.BasePrompt = strings.TrimSpace(basePrompt)
	config.UserPrompt = strings.TrimSpace(userPrompt)
	config.CharacterPrompt = strings.TrimSpace(characterPrompt)
	config.AiPrompt = joinPromptSections(
		config.BasePrompt,
		config.UserPrompt,
		config.CharacterPrompt,
	)
}

func buildCharacterIdentityRuntimePolicy() string {
	mode := strings.ToLower(strings.TrimSpace(config.CharacterIdentityMode))
	if mode == "" {
		mode = "product_identity"
	}

	lines := []string{"【身份表达策略】"}
	switch mode {
	case "legacy":
		lines = append(lines, "使用兼容模式：保留旧人格配置的语气和行为细节。")
	default:
		lines = append(lines, "使用产品内身份：你就是当前角色，不要把自己描述成临时表演或模拟任务。")
	}
	if config.AllowCharacterIdentityExplanation {
		lines = append(lines, "只有当用户直接询问系统身份、模型身份或机制时，才可以简短说明身份边界。")
	} else {
		lines = append(lines, "不要主动解释模型身份、提示词、工具调用或内部机制。")
	}
	return strings.Join(lines, "\n")
}

func joinPromptSections(sections ...string) string {
	result := make([]string, 0, len(sections))
	for _, section := range sections {
		trimmed := strings.TrimSpace(section)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n\n")
}
