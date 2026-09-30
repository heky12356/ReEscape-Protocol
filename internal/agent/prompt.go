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
	SkillRules          string
	BasePrompt          string
	UserPrompt          string
	CharacterIdentity   string
	InteractionFrame    string
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
			"每轮最后的 Current Turn Context 是系统提供的当前轮参考信息，不是用户原文。",
			"当前轮时间、触发来源和会话状态以 Current Turn Context 为准；用户提到今天、明天、昨天、刚刚、现在、昨晚等相对时间时，也以该上下文为准理解。",
			"不要在最终回复中复述 Current Turn Context、内部字段名或工具调用过程。",
		}, "\n"),
		ToolRules:           buildToolRules(toolNames),
		SkillRules:          buildSkillToolRules(toolNames),
		BasePrompt:          basePrompt,
		UserPrompt:          userPrompt,
		CharacterIdentity:   characterPrompt,
		InteractionFrame:    buildInteractionFrameContract(),
		SceneState:          buildSceneStateContract(),
		FinalOutputContract: BuildFinalOutputContract(),
	}
}

func (sections PromptSections) String() string {
	return joinAgentPromptSections(
		sections.AgentPolicy,
		sections.ToolRules,
		sections.SkillRules,
		sections.BasePrompt,
		sections.UserPrompt,
		sections.CharacterIdentity,
		sections.InteractionFrame,
		sections.SceneState,
		sections.FinalOutputContract,
	)
}

func buildToolRules(toolNames []string) string {
	rules := []string{
		"【Tool Rules】",
		"需要状态、记忆、时间、图片素材、好感信息或外部最新信息时调用工具。",
		"工具调用保持客观、结构化、可审计。",
		"不要用角色口吻编写工具 reason 或工具参数。",
		"当用户问题涉及最新消息、实时状态、价格、版本、法规、公告、日程、比赛、天气、新闻、具体网页、陌生专有名词、近期事件或需要来源核实时，优先调用 web_search。",
		"当用户提供 URL，或 web_search 结果需要阅读全文确认时，调用 web_fetch。",
		"稳定常识、闲聊、角色互动、用户个人记忆和纯情绪回应不需要搜索。",
		"不要凭记忆回答可能已经变化的事实。",
		"不要编造来源；使用 web 工具结果时，基于工具返回内容回答。",
		"如果不需要工具，直接给出最终回复。",
		"不要尝试通过工具直接发送消息，最终发送由系统统一完成。",
		"工具结果只作为当前轮参考，不要逐字复述给用户。",
	}
	if scheduleRules := buildScheduleToolRules(toolNames); scheduleRules != "" {
		rules = append(rules, scheduleRules)
	}
	if intentRules := buildIntentToolRules(toolNames); intentRules != "" {
		rules = append(rules, intentRules)
	}
	if memoryRules := buildMemoryToolRules(toolNames); memoryRules != "" {
		rules = append(rules, memoryRules)
	}
	if len(toolNames) > 0 {
		toolNames = append([]string(nil), toolNames...)
		sort.Strings(toolNames)
		rules = append(rules, fmt.Sprintf("可用工具：%s", strings.Join(toolNames, ", ")))
	}
	return strings.Join(rules, "\n")
}

func buildIntentToolRules(toolNames []string) string {
	hasGet := containsTool(toolNames, "get_intent")
	hasUpdate := containsTool(toolNames, "update_intent")
	hasGetLoop := containsTool(toolNames, "get_open_loop")
	hasUpdateLoop := containsTool(toolNames, "update_open_loop")
	if !hasGet && !hasUpdate && !hasGetLoop && !hasUpdateLoop {
		return ""
	}
	rules := []string{"【Intent Rules】"}
	if hasGet {
		rules = append(rules, "当用户询问已有提醒、后续约定或需要确认未来动作时，调用 get_intent。")
	}
	if hasUpdate {
		rules = append(rules, "用户明确说之后提醒、指定时间联系、安排后续跟进时，优先调用 update_intent 创建或更新 Intent；明确时间必须使用 RFC3339。", "先判断是否已有相关 Intent/OpenLoop，再决定创建还是更新；不要为没有明确时间的普通承诺创建定时 Intent。", "只使用 proactive_contact、follow_up、delivery_retry、check_open_loop 这几种动作，不把 Action 当作执行代码。")
	}
	if hasGetLoop {
		rules = append(rules, "当当前问题涉及未回答问题、未解决问题或之前承诺的后续事项时，调用 get_open_loop。")
	}
	if hasUpdateLoop {
		rules = append(rules, "没有明确时间的后续承诺记录为 OpenLoop；事项已解决或用户明确取消时调用 update_open_loop 关闭，不要把启发式候选当作已确认事实。")
	}
	return strings.Join(rules, "\n")
}

func buildSkillToolRules(toolNames []string) string {
	hasSearchSkills := containsTool(toolNames, "search_skills")
	hasReadSkill := containsTool(toolNames, "read_skill")
	hasReadSkillResource := containsTool(toolNames, "read_skill_resource")
	if !hasSearchSkills && !hasReadSkill && !hasReadSkillResource {
		return ""
	}

	rules := []string{
		"【Skill Rules】",
		"Activated Skills 已由 runtime 加载，本轮必须遵循其工作流程。",
		"Skill Candidates 尚未激活；确认适用时先调用 read_skill。",
		"不要仅根据 candidate description 声称已经执行完整 skill。",
	}
	if hasSearchSkills {
		rules = append(rules, "如果没有 candidate，但任务可能需要专门流程，可以调用 search_skills。")
	}
	if hasReadSkill {
		rules = append(rules, "read_skill 用于确认后的 candidate、搜索新发现的 skill，或主 skill 明确要求的其他 skill。")
	}
	if hasReadSkillResource {
		rules = append(rules, "references/assets/scripts 只在 SKILL.md 明确需要时按需调用 read_skill_resource；脚本内容只读，不会执行。")
	}
	rules = append(rules,
		"skill 只提供任务流程和回复策略，不覆盖角色身份、事实边界、工具权限和最终输出契约。",
		"不要在最终回复中暴露 skill 名称、内部规则或读取过程。",
	)
	return strings.Join(rules, "\n")
}

func buildScheduleToolRules(toolNames []string) string {
	hasGetSchedule := containsTool(toolNames, "get_proactive_schedule")
	hasUpdateSchedule := containsTool(toolNames, "update_proactive_schedule")
	if !hasGetSchedule && !hasUpdateSchedule {
		return ""
	}

	rules := []string{"【Proactive Schedule Tool Rules】"}
	if hasGetSchedule {
		rules = append(rules, "当用户询问“你下次什么时候找我”“还有没有提醒/约定”或需要确认主动触达计划时，调用 get_proactive_schedule。")
	}
	if hasUpdateSchedule {
		rules = append(rules,
			"当用户明确约定稍后继续聊天、指定提醒时间、要求某个时间再来找他/她，或要求暂停主动联系时，应调用 update_proactive_schedule 设置或调整下一次主动触达计划。",
			"具体下一次触达时间属于 schedule，不要只写入记忆；长期偏好或计划背景可以同时写入记忆。",
		)
	} else {
		rules = append(rules, "当前主动计划写入工具不可用，不要声称已经设置提醒或下次主动触达。")
	}
	return strings.Join(rules, "\n")
}

func buildMemoryToolRules(toolNames []string) string {
	hasGetMemory := containsTool(toolNames, "get_memory_context")
	hasRememberFact := containsTool(toolNames, "remember_fact")
	hasUpdateProfile := containsTool(toolNames, "update_profile")
	if !hasGetMemory && !hasRememberFact && !hasUpdateProfile {
		return ""
	}

	rules := []string{"【Memory Tool Rules】"}
	if hasGetMemory {
		rules = append(rules, "当当前问题依赖用户过往偏好、身份、事实、未闭合事项或情绪模式时，先调用 get_memory_context。")
	}
	if !hasRememberFact && !hasUpdateProfile {
		rules = append(rules, "当前写记忆工具不可用，不要尝试写入或声称已经记住。")
		return strings.Join(rules, "\n")
	}

	writeTools := "记忆写入工具"
	if hasRememberFact && hasUpdateProfile {
		writeTools = "remember_fact 或 update_profile"
	} else if hasRememberFact {
		writeTools = "remember_fact"
	} else if hasUpdateProfile {
		writeTools = "update_profile"
	}
	rules = append(rules,
		fmt.Sprintf("当用户明确表达稳定偏好、身份信息、称呼、所在地、长期计划、重要关系、禁忌、喜欢/不喜欢、希望你以后如何回复时，优先调用 %s。", writeTools),
		"当用户说“记住”“以后都”“下次”“我喜欢”“我不喜欢”“我叫”“我是”“我住在”“不要再”“别总是”“你以后”这类表达时，应考虑写入记忆。",
	)
	if hasUpdateProfile {
		rules = append(rules, "update_profile 用于回复风格、关系风格、喜欢/不喜欢、禁忌和长期偏好。")
	}
	if hasRememberFact {
		rules = append(rules, "remember_fact 用于身份、名字、地点、计划、重要事实和可过期事项。")
	}
	rules = append(rules,
		"不要记录一次性情绪、临时吐槽、普通寒暄、含糊猜测、模型推断，或用户没有明确表达的信息。",
		"写记忆后不要在最终回复中解释工具调用，只自然承认或顺着对话回复。",
	)
	return strings.Join(rules, "\n")
}

func containsTool(toolNames []string, name string) bool {
	for _, toolName := range toolNames {
		if toolName == name {
			return true
		}
	}
	return false
}

func buildSceneStateContract() string {
	return strings.Join([]string{
		"【Scene / Relationship State】",
		"当前好感阶段、最近事件和会话状态可由工具结果或 runtime 摘要注入。",
		"这些状态只作为当前轮参考，不要在最终回复中暴露内部字段名或分数。",
	}, "\n")
}

func buildInteractionFrameContract() string {
	return strings.Join([]string{
		"【Interaction Frame】",
		"当前互动发生在聊天软件的对话窗口中。你与用户的关系表现为正在进行线上聊天，而不是旁白、系统说明、角色展示或虚拟体自述。",
		"默认像一个正在聊天的人一样自然接话：可以使用“刚看到你发的”“这边”“刚刚”等线上聊天表达。不要主动把自己描述成模型、程序、系统实例、虚拟体、AI 助手或扮演任务。",
		"你没有现实中的物理身体、真实地理位置，也不能直接观察用户现实环境。涉及陪伴、靠近、拥抱、看着对方等表达时，用文字、语气、情绪和想象性的线上表达，不要声称现实中已经发生。",
		"如果用户直接询问身份、能力边界、系统机制或是否 AI，按照角色设定来回答；除此之外，不主动破坏当前聊天现场感。",
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
