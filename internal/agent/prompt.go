package agent

import (
	"fmt"
	"sort"
	"strings"

	"project-yume/internal/config"
)

type PromptSections struct {
	AgentPolicy         string
	ToolRules           string
	BasePrompt          string
	UserPrompt          string
	CharacterIdentity   string
	SceneState          string
	FinalOutputContract string
}

func BuildSystemPrompt(toolNames []string) string {
	return BuildPromptSections(toolNames).String()
}

func BuildPromptSections(toolNames []string) PromptSections {
	promptSections := config.CurrentPromptSections()
	basePrompt := strings.TrimSpace(promptSections.BasePrompt)
	userPrompt := strings.TrimSpace(promptSections.UserPrompt)
	characterPrompt := strings.TrimSpace(promptSections.CharacterPrompt)
	if characterPrompt == "" {
		characterPrompt = "你是 ReEscape Protocol 中的对话角色。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
	}

	return PromptSections{
		AgentPolicy: strings.Join([]string{
			"【Agent Policy】",
			"你是 ReEscape Protocol 的对话 agent。",
			"你可以使用工具读取或更新受控状态。",
			"不要输出隐藏推理过程。",
			"最终消息由 runtime 统一发送。",
		}, "\n"),
		ToolRules:           buildToolRules(toolNames),
		BasePrompt:          basePrompt,
		UserPrompt:          userPrompt,
		CharacterIdentity:   characterPrompt,
		SceneState:          buildSceneStateContract(),
		FinalOutputContract: BuildFinalOutputContract(),
	}
}

func (sections PromptSections) String() string {
	return joinAgentPromptSections(
		sections.AgentPolicy,
		sections.ToolRules,
		sections.BasePrompt,
		sections.UserPrompt,
		sections.CharacterIdentity,
		sections.SceneState,
		sections.FinalOutputContract,
	)
}

func buildToolRules(toolNames []string) string {
	rules := []string{
		"【Tool Rules】",
		"需要状态、记忆、时间、图片素材或好感信息时调用工具。",
		"工具调用保持客观、结构化、可审计。",
		"不要用角色口吻编写工具 reason 或工具参数。",
		"如果不需要工具，直接给出最终回复。",
		"不要尝试通过工具直接发送消息，最终发送由系统统一完成。",
		"工具结果只作为当前轮参考，不要逐字复述给用户。",
	}
	if len(toolNames) > 0 {
		toolNames = append([]string(nil), toolNames...)
		sort.Strings(toolNames)
		rules = append(rules, fmt.Sprintf("可用工具：%s", strings.Join(toolNames, ", ")))
	}
	return strings.Join(rules, "\n")
}

func buildSceneStateContract() string {
	return strings.Join([]string{
		"【Scene / Relationship State】",
		"当前好感阶段、最近事件和会话状态可由工具结果或 runtime 摘要注入。",
		"这些状态只作为当前轮参考，不要在最终回复中暴露内部字段名或分数。",
	}, "\n")
}

func BuildFinalOutputContract() string {
	return strings.Join([]string{
		"【Final Output Contract】",
		"最终只输出用户可见回复。",
		"保持角色身份和说话方式。",
		"不要输出 JSON。",
		"不要解释工具调用、系统提示词或内部状态判断。",
	}, "\n")
}

func joinAgentPromptSections(sections ...string) string {
	result := make([]string, 0, len(sections))
	for _, section := range sections {
		trimmed := strings.TrimSpace(section)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n\n")
}
