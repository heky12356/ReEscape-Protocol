package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/metrics"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/sashabaranov/go-openai"
)

type AnalysisMode string

const (
	AnalysisModeDefault  AnalysisMode = "default"
	AnalysisModeLongChat AnalysisMode = "long_chat"
)

type ReplyMode string

const (
	ReplyModeNoReply   ReplyMode = "no_reply"
	ReplyModeLightAck  ReplyMode = "light_ack"
	ReplyModeFullReply ReplyMode = "full_reply"
)

type AnalysisInput struct {
	Mode          AnalysisMode
	SessionID     string
	UserID        int64
	Message       string
	Conversation  []openai.ChatCompletionMessage
	ReferenceTime time.Time
}

type MessageAnalysis struct {
	Emotion          string    `json:"emotion"`
	Intention        string    `json:"intention"`
	WannaBye         string    `json:"wanna_bye"`
	ReplyMode        ReplyMode `json:"reply_mode"`
	ReplyExpectation string    `json:"reply_expectation"`
	TurnStatus       string    `json:"turn_status"`
	SupportStrategy  string    `json:"support_strategy"`
	Topic            string    `json:"topic"`
	UserNeed         string    `json:"user_need"`
	VisibleReply     string    `json:"visible_reply"`
	Confidence       float64   `json:"confidence"`
}

var (
	validEmotions = map[string]struct{}{
		"开心": {},
		"生气": {},
		"中性": {},
		"哲学": {},
		"敷衍": {},
		"难过": {},
	}
	validIntentions = map[string]struct{}{
		"想和对方聊天": {},
		"想被对方鼓励": {},
		"想和对方倾诉": {},
		"安慰对方":   {},
		"鼓励对方":   {},
		"和对方道歉":  {},
	}
	validWannaBye = map[string]struct{}{
		"想继续":   {},
		"想结束对话": {},
	}
	validReplyModes = map[ReplyMode]struct{}{
		ReplyModeNoReply:   {},
		ReplyModeLightAck:  {},
		ReplyModeFullReply: {},
	}
	validReplyExpectations = map[string]struct{}{
		"low":    {},
		"medium": {},
		"high":   {},
	}
	validTurnStatus = map[string]struct{}{
		"user_holds_floor": {},
		"handoff_to_ai":    {},
	}
	validSupportStrategy = map[string]struct{}{
		"acknowledge_and_wait": {},
		"comfort":              {},
		"encourage":            {},
		"answer_directly":      {},
		"continue_chat":        {},
		"close_conversation":   {},
	}
)

// AnalyzeMessage 单次调用模型，返回结构化分析与回复决策。
func AnalyzeMessage(input AnalysisInput) (MessageAnalysis, error) {
	startedAt := time.Now()
	defer func() {
		metrics.ObserveDuration(
			"bot_ai_request_duration",
			"AI request duration.",
			time.Since(startedAt),
			map[string]string{"kind": "classify_reply", "mode": string(input.Mode)},
		)
	}()

	prompt := buildAnalysisPrompt(input)
	raw, err := aifunction.Queryai(prompt, buildAnalysisPayload(input))
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "classify_reply", "mode": string(input.Mode), "result": "fallback_ai"},
		)
		utils.Warn("AnalyzeMessage fallback due to AI error: %v", err)
		return fallbackAnalysis(input.Mode), nil
	}

	result, err := parseMessageAnalysis(raw, input.Mode)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "classify_reply", "mode": string(input.Mode), "result": "fallback_parse"},
		)
		utils.Warn("AnalyzeMessage fallback due to parse error: %v, raw=%q", err, raw)
		return fallbackAnalysis(input.Mode), nil
	}

	metrics.IncCounter(
		"bot_ai_requests_total",
		"Total AI requests by kind and result.",
		map[string]string{"kind": "classify_reply", "mode": string(input.Mode), "result": "ok"},
	)
	return result, nil
}

func buildAnalysisPrompt(input AnalysisInput) string {
	cfg := config.GetConfig()
	baseRules := `
你是一个对话引擎的内部决策器。你需要同时完成：
1. 分析用户当前说话方式与情绪
2. 判断此刻是否该回复，以及回复强度
3. 生成最终给用户看到的 visible_reply

请严格返回 JSON，不要输出解释，不要使用 markdown 代码块。

字段要求：
- emotion: 只能是 [开心, 生气, 中性, 哲学, 敷衍, 难过]
- intention: 只能是 [想和对方聊天, 想被对方鼓励, 想和对方倾诉, 安慰对方, 鼓励对方, 和对方道歉]
- wanna_bye: 只能是 [想继续, 想结束对话]
- reply_mode: 只能是 [no_reply, light_ack, full_reply]
- reply_expectation: 只能是 [low, medium, high]
- turn_status: 只能是 [user_holds_floor, handoff_to_ai]
- support_strategy: 只能是 [acknowledge_and_wait, comfort, encourage, answer_directly, continue_chat, close_conversation]
- topic: 用一句短语概括当前主题，没有就返回空字符串
- user_need: 用一句短语概括当前更像需要什么，没有就返回空字符串
- confidence: 0 到 1 之间的小数
- visible_reply: 给用户看的最终回复文本

额外要求：
- 所有字段都必须出现。
- 如果意图不明确，intention 选择 "想和对方聊天"。
- 如果情感不明确，emotion 选择 "中性"。
- 如果用户更像在继续表达自己、补充观点、并未明显把话头交给你，可以选择 no_reply 或 light_ack。
- no_reply 时 visible_reply 必须为空字符串。
- light_ack 时 visible_reply 控制在 2 到 12 个字，不主动展开新话题，作用是接住但不抢话。
- full_reply 时 visible_reply 才能正常展开。
- visible_reply 必须符合角色设定和输出风格要求。
- 只返回单个 JSON 对象。

角色与回复风格要求：
` + cfg.AiPrompt + `
`

	if input.Mode == AnalysisModeLongChat {
		return baseRules + `
当前场景：用户正在和 AI 进行长对话。

wanna_bye 判断标准：
- "想结束对话"：再见、拜拜、晚安、我走了、先这样吧、不聊了、谢谢你今天陪我聊天、困了要睡觉了 等
- "想继续"：提出新话题、继续追问、简单回应、等等、稍等、语气词、表情等
- 当意图不明确时，必须选择 "想继续"，避免误结束对话。

reply_mode 额外要求：
- 如果用户在延续自我表达，只需轻微附和，优先 light_ack。
- 如果用户明确提问、求安慰、求建议、求判断，优先 full_reply。
- 如果用户只是补充上一句并且明显还没说完，可以 no_reply。

输出示例：
{"emotion":"开心","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"light_ack","reply_expectation":"medium","turn_status":"user_holds_floor","support_strategy":"acknowledge_and_wait","topic":"最近的烦恼","user_need":"被倾听","confidence":0.82,"visible_reply":"嗯，你继续说。"}
`
	}

	return baseRules + `
当前场景：普通消息处理。

wanna_bye 默认返回 "想继续"。

reply_mode 判断标准：
- 用户明显在自我延续、补充观点、没有直接向你发问时，可选 no_reply 或 light_ack。
- 用户明确寻求回应时，选 full_reply。

输出示例：
{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"full_reply","reply_expectation":"high","turn_status":"handoff_to_ai","support_strategy":"continue_chat","topic":"今天的安排","user_need":"被回应","confidence":0.76,"visible_reply":"听起来你今天事情还不少。$你最想先处理哪件？"}
`
}

func parseMessageAnalysis(raw string, mode AnalysisMode) (MessageAnalysis, error) {
	jsonText, err := extractFirstJSONObject(raw)
	if err != nil {
		return MessageAnalysis{}, err
	}

	var result MessageAnalysis
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return MessageAnalysis{}, fmt.Errorf("unmarshal analysis json failed: %w", err)
	}

	result.Emotion = strings.TrimSpace(result.Emotion)
	result.Intention = strings.TrimSpace(result.Intention)
	result.WannaBye = strings.TrimSpace(result.WannaBye)
	result.ReplyMode = ReplyMode(strings.TrimSpace(string(result.ReplyMode)))
	result.ReplyExpectation = strings.TrimSpace(result.ReplyExpectation)
	result.TurnStatus = strings.TrimSpace(result.TurnStatus)
	result.SupportStrategy = strings.TrimSpace(result.SupportStrategy)
	result.Topic = strings.TrimSpace(result.Topic)
	result.UserNeed = strings.TrimSpace(result.UserNeed)
	result.VisibleReply = strings.TrimSpace(result.VisibleReply)

	if mode != AnalysisModeLongChat && result.WannaBye == "" {
		result.WannaBye = "想继续"
	}

	if err := validateMessageAnalysis(result, mode); err != nil {
		return MessageAnalysis{}, err
	}

	return result, nil
}

func extractFirstJSONObject(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	start := strings.Index(trimmed, "{")
	if start < 0 {
		return "", fmt.Errorf("no json object start found")
	}

	depth := 0
	for i := start; i < len(trimmed); i++ {
		switch trimmed[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return trimmed[start : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("no complete json object found")
}

func validateMessageAnalysis(result MessageAnalysis, mode AnalysisMode) error {
	if _, ok := validEmotions[result.Emotion]; !ok {
		return fmt.Errorf("invalid emotion: %q", result.Emotion)
	}
	if _, ok := validIntentions[result.Intention]; !ok {
		return fmt.Errorf("invalid intention: %q", result.Intention)
	}
	if _, ok := validWannaBye[result.WannaBye]; !ok {
		return fmt.Errorf("invalid wanna_bye: %q", result.WannaBye)
	}
	if mode != AnalysisModeLongChat && result.WannaBye != "想继续" {
		return fmt.Errorf("unexpected wanna_bye for default mode: %q", result.WannaBye)
	}
	if _, ok := validReplyModes[result.ReplyMode]; !ok {
		return fmt.Errorf("invalid reply_mode: %q", result.ReplyMode)
	}
	if _, ok := validReplyExpectations[result.ReplyExpectation]; !ok {
		return fmt.Errorf("invalid reply_expectation: %q", result.ReplyExpectation)
	}
	if _, ok := validTurnStatus[result.TurnStatus]; !ok {
		return fmt.Errorf("invalid turn_status: %q", result.TurnStatus)
	}
	if _, ok := validSupportStrategy[result.SupportStrategy]; !ok {
		return fmt.Errorf("invalid support_strategy: %q", result.SupportStrategy)
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return fmt.Errorf("invalid confidence: %v", result.Confidence)
	}
	if result.ReplyMode == ReplyModeNoReply && result.VisibleReply != "" {
		return fmt.Errorf("visible_reply must be empty when reply_mode=no_reply")
	}
	if result.ReplyMode == ReplyModeLightAck {
		runes := []rune(result.VisibleReply)
		if len(runes) == 0 || len(runes) > 12 {
			return fmt.Errorf("light_ack visible_reply length invalid: %q", result.VisibleReply)
		}
	}
	if result.ReplyMode == ReplyModeFullReply && result.VisibleReply == "" {
		return fmt.Errorf("visible_reply must not be empty when reply_mode=full_reply")
	}
	return nil
}

func fallbackAnalysis(mode AnalysisMode) MessageAnalysis {
	result := MessageAnalysis{
		Emotion:          "中性",
		Intention:        "想和对方聊天",
		WannaBye:         "想继续",
		ReplyMode:        ReplyModeLightAck,
		ReplyExpectation: "medium",
		TurnStatus:       "handoff_to_ai",
		SupportStrategy:  "continue_chat",
		Topic:            "",
		UserNeed:         "",
		VisibleReply:     "嗯。",
		Confidence:       0.3,
	}
	if mode == AnalysisModeLongChat {
		return result
	}
	return result
}

func buildAnalysisPayload(input AnalysisInput) string {
	sections := make([]string, 0, 5)
	if timeContext := BuildTimeContext(input.ReferenceTime); timeContext != "" {
		sections = append(sections, timeContext)
	}
	if dialogueStateContext := buildDialogueStatePromptContext(input.SessionID); dialogueStateContext != "" {
		sections = append(sections, dialogueStateContext)
	}
	if memoryContext := FormatPromptMemory(BuildPromptMemory(input.UserID, input.SessionID, input.Message)); memoryContext != "" {
		sections = append(sections, memoryContext)
	}

	sections = append(sections, "【最近对话】\n"+formatRecentConversation(input.Conversation))
	sections = append(sections, "【当前用户消息】\n"+strings.TrimSpace(input.Message))
	return strings.Join(sections, "\n\n")
}

func buildDialogueStatePromptContext(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}

	dialogueState := state.GetManager().GetDialogueState(sessionID)
	lines := make([]string, 0, 7)
	if dialogueState.Emotion != "" {
		lines = append(lines, "上轮判断情绪："+dialogueState.Emotion)
	}
	if dialogueState.Intention != "" {
		lines = append(lines, "上轮判断意图："+dialogueState.Intention)
	}
	if dialogueState.ReplyExpectation != "" {
		lines = append(lines, "上轮回复期待："+dialogueState.ReplyExpectation)
	}
	if dialogueState.TurnStatus != "" {
		lines = append(lines, "上轮话轮状态："+dialogueState.TurnStatus)
	}
	if dialogueState.SupportStrategy != "" {
		lines = append(lines, "上轮支持策略："+dialogueState.SupportStrategy)
	}
	if dialogueState.Topic != "" {
		lines = append(lines, "上轮主题："+dialogueState.Topic)
	}
	if dialogueState.UserNeed != "" {
		lines = append(lines, "上轮推测需求："+dialogueState.UserNeed)
	}
	if len(lines) == 0 {
		return ""
	}
	return "【最近隐藏对话状态】\n" + strings.Join(lines, "\n")
}

func formatRecentConversation(conversation []openai.ChatCompletionMessage) string {
	if len(conversation) == 0 {
		return "暂无"
	}

	start := 0
	if len(conversation) > 8 {
		start = len(conversation) - 8
	}

	lines := make([]string, 0, len(conversation)-start)
	for _, msg := range conversation[start:] {
		if msg.Role == "system" {
			continue
		}
		content := strings.TrimSpace(chatMessagePlainTextForAnalysis(msg))
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", summarizeRoleForAnalysis(msg.Role), content))
	}
	if len(lines) == 0 {
		return "暂无"
	}
	return strings.Join(lines, "\n")
}

func chatMessagePlainTextForAnalysis(msg openai.ChatCompletionMessage) string {
	if strings.TrimSpace(msg.Content) != "" {
		return msg.Content
	}
	if len(msg.MultiContent) == 0 {
		return ""
	}

	parts := make([]string, 0, len(msg.MultiContent))
	for _, part := range msg.MultiContent {
		switch part.Type {
		case openai.ChatMessagePartTypeText:
			text := strings.TrimSpace(part.Text)
			if text != "" {
				parts = append(parts, text)
			}
		case openai.ChatMessagePartTypeImageURL:
			parts = append(parts, "[图片]")
		}
	}

	return strings.Join(parts, " ")
}

func summarizeRoleForAnalysis(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	default:
		return role
	}
}

// EnhancePromptWithMemory 基于分层记忆增强AI提示词
func EnhancePromptWithMemory(userID int64, sessionID, originalPrompt, currentMessage string, referenceTime time.Time) string {
	contexts := make([]string, 0, 2)

	if timeContext := BuildTimeContext(referenceTime); timeContext != "" {
		contexts = append(contexts, timeContext)
	}
	if imageAssetContext := BuildImageAssetPromptContext(currentMessage); imageAssetContext != "" {
		contexts = append(contexts, imageAssetContext)
	}
	if dialogueStateContext := buildDialogueStatePromptContext(sessionID); dialogueStateContext != "" {
		contexts = append(contexts, dialogueStateContext)
	}

	memoryContext := FormatPromptMemory(BuildPromptMemory(userID, sessionID, currentMessage))
	if memoryContext != "" {
		contexts = append(contexts, memoryContext)
	}

	if len(contexts) == 0 {
		return originalPrompt
	}

	return originalPrompt + "\n\n" + strings.Join(contexts, "\n\n")
}

// BuildEmotionalContext 构建情感上下文
func BuildEmotionalContext(pattern string, recentEmotions []string) string {
	context := "【用户情感档案】\n"

	// 添加对话模式信息
	switch pattern {
	case "需要关怀":
		context += "用户当前情感状态：需要关怀和安慰，请用温暖、体贴的语气回复，多表达理解和支持。\n"
	case "积极活跃":
		context += "用户当前情感状态：积极活跃，请用轻松、愉快的语气回复，可以适当幽默和活泼。\n"
	case "情绪波动":
		context += "用户当前情感状态：情绪不太稳定，请用平和、耐心的语气回复，避免过于激烈的表达。\n"
	case "新用户":
		context += "用户情感状态：新用户，请用友善、欢迎的语气回复，帮助用户熟悉对话。\n"
	default:
		context += "用户情感状态：平稳交流，请保持自然、友好的对话风格。\n"
	}

	// 添加最近情感趋势
	if len(recentEmotions) > 0 {
		context += "最近情感趋势：" + strings.Join(recentEmotions, " → ") + "\n"

		// 分析情感变化
		if len(recentEmotions) >= 2 {
			lastEmotion := recentEmotions[len(recentEmotions)-1]
			prevEmotion := recentEmotions[len(recentEmotions)-2]

			if lastEmotion != prevEmotion {
				context += "注意：用户情感刚刚从「" + prevEmotion + "」变为「" + lastEmotion + "」，请关注这个变化。\n"
			}
		}
	}

	context += "\n请根据以上情感档案调整你的回复风格和内容，让对话更贴近用户的情感需求。"

	return context
}

// UpdateSystemPromptWithMemory 基于分层记忆更新系统提示词
func UpdateSystemPromptWithMemory(userID int64, sessionID, currentMessage string, referenceTime time.Time, conversation []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	contextSections := make([]string, 0, 2)

	if timeContext := BuildTimeContext(referenceTime); timeContext != "" {
		contextSections = append(contextSections, timeContext)
	}
	if imageAssetContext := BuildImageAssetPromptContext(currentMessage); imageAssetContext != "" {
		contextSections = append(contextSections, imageAssetContext)
	}
	if dialogueStateContext := buildDialogueStatePromptContext(sessionID); dialogueStateContext != "" {
		contextSections = append(contextSections, dialogueStateContext)
	}

	memoryContext := FormatPromptMemory(BuildPromptMemory(userID, sessionID, currentMessage))
	if memoryContext != "" {
		contextSections = append(contextSections, "【记忆增强】\n"+memoryContext)
	}

	if len(contextSections) == 0 {
		return conversation
	}

	memoryMessage := openai.ChatCompletionMessage{
		Role:    "system",
		Content: strings.Join(contextSections, "\n\n"),
	}

	for i, msg := range conversation {
		if msg.Role == "system" && (strings.Contains(msg.Content, "【记忆增强】") || strings.Contains(msg.Content, "【时间上下文】")) {
			conversation[i] = memoryMessage
			return conversation
		}
	}

	updated := make([]openai.ChatCompletionMessage, 0, len(conversation)+1)
	inserted := false
	for _, msg := range conversation {
		updated = append(updated, msg)
		if !inserted && msg.Role == "system" {
			updated = append(updated, memoryMessage)
			inserted = true
		}
	}
	if !inserted {
		updated = append([]openai.ChatCompletionMessage{memoryMessage}, updated...)
	}
	return updated
}

// BuildEmotionalUpdate 构建情感更新信息
func BuildEmotionalUpdate(pattern string, recentEmotions []string) string {
	if len(recentEmotions) == 0 {
		return ""
	}

	update := "【情感状态更新】\n"

	// 根据对话模式提供指导
	switch pattern {
	case "需要关怀":
		update += "用户当前需要更多关怀，请在回复中体现温暖和理解。\n"
	case "积极活跃":
		update += "用户情绪积极，可以保持轻松愉快的对话氛围。\n"
	case "情绪波动":
		update += "用户情绪有波动，请保持耐心和稳定的回复风格。\n"
	}

	// 分析最近情感变化
	if len(recentEmotions) >= 2 {
		lastEmotion := recentEmotions[len(recentEmotions)-1]
		prevEmotion := recentEmotions[len(recentEmotions)-2]

		if lastEmotion != prevEmotion {
			update += "注意：用户情感从「" + prevEmotion + "」变为「" + lastEmotion + "」，请适当调整回复风格。\n"
		}
	}

	update += "最近情感：" + strings.Join(recentEmotions, " → ")

	return update
}
