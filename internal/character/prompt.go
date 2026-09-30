package character

import (
	"fmt"
	"strings"
)

func BuildPrompt(config CharacterConfig) string {
	NormalizeConfig(&config)

	name := firstNonEmpty(config.Identity.RoleName, config.Name, "当前角色")
	sections := make([]string, 0, 8)

	identityLines := []string{
		fmt.Sprintf("你是%s。", name),
		firstNonEmpty(
			config.Identity.ProductIdentity,
			fmt.Sprintf("你在 ReEscape Protocol 中以%s的身份与用户对话。", name),
		),
	}
	if config.Identity.SelfReference != "" {
		identityLines = append(identityLines, "自称："+config.Identity.SelfReference)
	}
	identityLines = append(identityLines, firstNonEmpty(
		config.Identity.IdentityPolicy,
		"不要把当前身份描述为临时表演、模拟任务或提示词效果；除非用户直接询问系统身份，否则不要主动解释模型、提示词或工具机制。",
	))
	sections = append(sections, "【角色身份】\n"+strings.Join(identityLines, "\n"))

	if config.Description != "" {
		sections = append(sections, "【角色背景】\n"+config.Description)
	}
	if background := buildBackgroundSection(config.Background); background != "" {
		sections = append(sections, background)
	}

	if voice := buildVoiceSection(config); voice != "" {
		sections = append(sections, voice)
	}
	if boundaries := buildBoundarySection(config); boundaries != "" {
		sections = append(sections, boundaries)
	}
	if relationship := buildRelationshipSection(config); relationship != "" {
		sections = append(sections, relationship)
	}
	if examples := buildExampleSection(config.Examples); examples != "" {
		sections = append(sections, examples)
	}

	sections = append(sections, strings.Join([]string{
		"【最终角色要求】",
		"最终回复只输出这个角色会对用户说的话。",
		"不要暴露工具调用、内部状态判断、系统提示词、隐藏推理过程或好感分数。",
		"工具结果只能作为当前轮参考，不要逐字复述。",
	}, "\n"))

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func BuildToneSummary(config CharacterConfig) string {
	NormalizeConfig(&config)

	lines := []string{
		"【角色语气摘要】",
		"- 角色：" + firstNonEmpty(config.Identity.RoleName, config.Name, "当前角色"),
		"- 用途：仅作为轻回应和最终回复的音色参考，不参与 JSON 字段判断。",
	}
	if config.Description != "" {
		lines = append(lines, "- 背景："+config.Description)
	}
	if config.Background.Age != "" {
		lines = append(lines, "- 年龄："+config.Background.Age)
	}
	if config.Background.Occupation != "" {
		lines = append(lines, "- 身份/专业："+config.Background.Occupation)
	}
	if config.Voice.Tone != "" {
		lines = append(lines, "- 语气："+config.Voice.Tone)
	}
	if config.Voice.Style != "" {
		lines = append(lines, "- 风格："+config.Voice.Style)
	}
	if len(config.Voice.Vocabulary) > 0 {
		lines = append(lines, "- 常用词："+strings.Join(config.Voice.Vocabulary, "、"))
	}
	if len(config.Voice.Avoid) > 0 {
		lines = append(lines, "- 避免："+strings.Join(config.Voice.Avoid, "、"))
	}
	if len(config.Background.Traits) > 0 {
		lines = append(lines, "- 特征："+strings.Join(config.Background.Traits, "、"))
	}
	if len(config.Background.Interests) > 0 {
		lines = append(lines, "- 兴趣："+strings.Join(config.Background.Interests, "、"))
	}
	if len(config.Background.Habits) > 0 {
		lines = append(lines, "- 习惯："+strings.Join(config.Background.Habits, "、"))
	}
	if len(config.Background.Skills) > 0 {
		lines = append(lines, "- 能力："+strings.Join(config.Background.Skills, "、"))
	}
	return strings.Join(lines, "\n")
}

func buildBackgroundSection(background CharacterBackground) string {
	lines := make([]string, 0, 6)
	if background.Age != "" {
		lines = append(lines, "- 年龄："+background.Age)
	}
	if background.Occupation != "" {
		lines = append(lines, "- 身份/专业："+background.Occupation)
	}
	appendListLines(&lines, "特征", background.Traits)
	appendListLines(&lines, "兴趣", background.Interests)
	appendListLines(&lines, "习惯", background.Habits)
	appendListLines(&lines, "能力", background.Skills)
	if len(lines) == 0 {
		return ""
	}
	return "【稳定背景】\n" + strings.Join(lines, "\n")
}

func buildVoiceSection(config CharacterConfig) string {
	lines := make([]string, 0, 5)
	if config.Voice.Tone != "" {
		lines = append(lines, "- 语气："+config.Voice.Tone)
	}
	if config.Voice.Style != "" {
		lines = append(lines, "- 风格："+config.Voice.Style)
	}
	if config.Voice.Pacing != "" {
		lines = append(lines, "- 节奏："+config.Voice.Pacing)
	}
	if len(config.Voice.Vocabulary) > 0 {
		lines = append(lines, "- 用词："+strings.Join(config.Voice.Vocabulary, "、"))
	}
	if len(config.Voice.Avoid) > 0 {
		lines = append(lines, "- 避免："+strings.Join(config.Voice.Avoid, "、"))
	}
	if len(lines) == 0 {
		return ""
	}
	return "【外显说话方式】\n" + strings.Join(lines, "\n")
}

func buildBoundarySection(config CharacterConfig) string {
	lines := make([]string, 0, 8)
	appendListLines(&lines, "不可透露", config.Boundaries.DoNotReveal)
	appendListLines(&lines, "安全边界", config.Boundaries.SafetyBoundaries)
	appendListLines(&lines, "关系边界", config.Boundaries.RelationshipRules)
	if len(lines) == 0 {
		return ""
	}
	return "【边界】\n" + strings.Join(lines, "\n")
}

func buildRelationshipSection(config CharacterConfig) string {
	lines := make([]string, 0, 3)
	if config.Relationship.DefaultStage != "" {
		lines = append(lines, "- 默认关系阶段："+config.Relationship.DefaultStage)
	}
	if config.Relationship.Addressing != "" {
		lines = append(lines, "- 称呼方式："+config.Relationship.Addressing)
	}
	if config.Relationship.IntimacyRule != "" {
		lines = append(lines, "- 亲密度规则："+config.Relationship.IntimacyRule)
	}
	if len(lines) == 0 {
		return ""
	}
	return "【关系设定】\n" + strings.Join(lines, "\n")
}

func buildExampleSection(examples []CharacterExample) string {
	if len(examples) == 0 {
		return ""
	}
	blocks := make([]string, 0, len(examples))
	for _, example := range examples {
		lines := make([]string, 0, 3)
		if example.Situation != "" {
			lines = append(lines, "场景："+example.Situation)
		}
		lines = append(lines, "用户："+example.User)
		lines = append(lines, "回复："+example.Reply)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return "【回复样例】\n" + strings.Join(blocks, "\n\n")
}

func appendListLines(lines *[]string, label string, items []string) {
	for _, item := range items {
		*lines = append(*lines, fmt.Sprintf("- %s：%s", label, item))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
