package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/metrics"
	"project-yume/internal/utils"

	"github.com/sashabaranov/go-openai"
)

type AnalysisMode string

const (
	AnalysisModeDefault  AnalysisMode = "default"
	AnalysisModeLongChat AnalysisMode = "long_chat"
)

type MessageAnalysis struct {
	Emotion   string `json:"emotion"`
	Intention string `json:"intention"`
	WannaBye  string `json:"wanna_bye"`
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
)

// AnalyzeMessage 单次调用模型，返回结构化分类结果。
func AnalyzeMessage(message string, mode AnalysisMode) (MessageAnalysis, error) {
	startedAt := time.Now()
	defer func() {
		metrics.ObserveDuration(
			"bot_ai_request_duration",
			"AI request duration.",
			time.Since(startedAt),
			map[string]string{"kind": "classify", "mode": string(mode)},
		)
	}()

	prompt := buildAnalysisPrompt(mode)
	raw, err := aifunction.Queryai(prompt, message)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "classify", "mode": string(mode), "result": "fallback_ai"},
		)
		utils.Warn("AnalyzeMessage fallback due to AI error: %v", err)
		return fallbackAnalysis(mode), nil
	}

	result, err := parseMessageAnalysis(raw, mode)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "classify", "mode": string(mode), "result": "fallback_parse"},
		)
		utils.Warn("AnalyzeMessage fallback due to parse error: %v, raw=%q", err, raw)
		return fallbackAnalysis(mode), nil
	}

	metrics.IncCounter(
		"bot_ai_requests_total",
		"Total AI requests by kind and result.",
		map[string]string{"kind": "classify", "mode": string(mode), "result": "ok"},
	)
	return result, nil
}

func buildAnalysisPrompt(mode AnalysisMode) string {
	baseRules := `
你是一个消息分类器。请严格返回 JSON，不要输出解释，不要使用 markdown 代码块。

字段要求：
- emotion: 只能是 [开心, 生气, 中性, 哲学, 敷衍, 难过]
- intention: 只能是 [想和对方聊天, 想被对方鼓励, 想和对方倾诉, 安慰对方, 鼓励对方, 和对方道歉]
- wanna_bye: 只能是 [想继续, 想结束对话]

额外要求：
- 所有字段都必须出现。
- 如果意图不明确，intention 选择 "想和对方聊天"。
- 如果情感不明确，emotion 选择 "中性"。
- 只返回单个 JSON 对象。
`

	if mode == AnalysisModeLongChat {
		return baseRules + `
当前场景：用户正在和 AI 进行长对话。

wanna_bye 判断标准：
- "想结束对话"：再见、拜拜、晚安、我走了、先这样吧、不聊了、谢谢你今天陪我聊天、困了要睡觉了 等
- "想继续"：提出新话题、继续追问、简单回应、等等、稍等、语气词、表情等
- 当意图不明确时，必须选择 "想继续"，避免误结束对话。

输出示例：
{"emotion":"开心","intention":"想和对方聊天","wanna_bye":"想继续"}
`
	}

	return baseRules + `
当前场景：普通消息分类，不在结束对话判断场景。

wanna_bye 一律返回 "想继续"。

输出示例：
{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续"}
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
	return nil
}

func fallbackAnalysis(mode AnalysisMode) MessageAnalysis {
	result := MessageAnalysis{
		Emotion:   "中性",
		Intention: "想和对方聊天",
		WannaBye:  "想继续",
	}
	if mode == AnalysisModeLongChat {
		return result
	}
	return result
}

// EnhancePromptWithMemory 基于分层记忆增强AI提示词
func EnhancePromptWithMemory(userID int64, sessionID, originalPrompt, currentMessage string) string {
	memoryContext := FormatPromptMemory(BuildPromptMemory(userID, sessionID, currentMessage))
	if memoryContext == "" {
		return originalPrompt
	}

	return originalPrompt + "\n\n" + memoryContext
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
func UpdateSystemPromptWithMemory(userID int64, sessionID, currentMessage string, conversation []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	memoryContext := FormatPromptMemory(BuildPromptMemory(userID, sessionID, currentMessage))
	if memoryContext == "" {
		return conversation
	}

	memoryMessage := openai.ChatCompletionMessage{
		Role:    "system",
		Content: "【记忆增强】\n" + memoryContext,
	}

	for i, msg := range conversation {
		if msg.Role == "system" && strings.Contains(msg.Content, "【记忆增强】") {
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
