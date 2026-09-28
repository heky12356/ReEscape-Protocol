package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/agent"
	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/metrics"
	"project-yume/internal/model"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

const aiFallbackReply = "?"

type MessageContext struct {
	RequestID                  string
	SessionID                  string
	UserID                     int64
	GroupID                    int64
	ChatType                   int
	MessageID                  int64
	MessageIDs                 []int64
	RawSegments                []string
	RawSegmentTimes            []int64
	Parts                      []model.MessagePart
	Aggregated                 bool
	SegmentCount               int
	RawMessage                 string
	Message                    string
	ReceivedAt                 time.Time
	StartedAt                  time.Time
	EndedAt                    time.Time
	PreviousUserMessageAt      time.Time
	PreviousAssistantMessageAt time.Time
	PreviousInteractionAt      time.Time
	DropReason                 string
}

func sendAIFallbackReply(c *websocket.Conn, ctx MessageContext) (string, error) {
	return sendContextReply(c, ctx, aiFallbackReply)
}

func sendContextReply(c *websocket.Conn, ctx MessageContext, reply string) (string, error) {
	result := service.DeliverTurnReply(context.Background(), c, service.DeliveryRequest{
		TurnID: ctx.RequestID, SessionID: ctx.SessionID, UserID: ctx.UserID, Reply: reply,
	})
	if result.Status == service.DeliveryResultDelivered || result.Status == service.DeliveryResultPartial {
		return result.DeliveredContent, nil
	}
	if result.Error != "" {
		return "", fmt.Errorf("reply delivery %s: %s", result.Status, result.Error)
	}
	return "", fmt.Errorf("reply delivery %s", result.Status)
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	CanHandle(ctx MessageContext, sm *state.StateManager) bool
	Handle(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (*ProcessResult, error)
}

// PresetHandler 预设回复处理器
type PresetHandler struct {
	responses map[string]string
}

func NewPresetHandler() *PresetHandler {
	return &PresetHandler{
		responses: map[string]string{
			"你好":     "你好",
			"在干嘛":    "在学习",
			"在忙呢":    "好吧",
			"难过了":    "别难过，开心点，加油！",
			"我想你了":   "是嘛？嘿嘿",
			"能陪我聊聊吗": "好",
			"晚安":     "晚安",
			"早安":     "早安",
		},
	}
}

// CanHandle 检查是否可以处理消息
func (h *PresetHandler) CanHandle(ctx MessageContext, sm *state.StateManager) bool {
	if sm.GetState(ctx.SessionID) != state.StateIdle {
		return false
	}
	_, exists := h.responses[ctx.Message]
	return exists
}

// Handle 处理消息
func (h *PresetHandler) Handle(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (*ProcessResult, error) {
	response := h.responses[ctx.Message]

	if ctx.Message == "在忙呢" {
		sm.SetState(ctx.SessionID, state.StateBusy)
	}
	if ctx.Message == "能陪我聊聊吗" {
		sm.SetState(ctx.SessionID, state.StateLongChat)
	}

	if _, err := sendContextReply(c, ctx, response); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Handled:   true,
		Replied:   true,
		ReplyMode: service.ReplyModeFullReply,
		Reply:     response,
	}, nil
}

// EmotionHandler 情感分析处理器
type EmotionHandler struct{}

func NewEmotionHandler() *EmotionHandler {
	return &EmotionHandler{}
}

func (h *EmotionHandler) CanHandle(ctx MessageContext, sm *state.StateManager) bool {
	return sm.GetState(ctx.SessionID) == state.StateIdle
}

func (h *EmotionHandler) Handle(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (*ProcessResult, error) {
	analysis, err := service.AnalyzeMessage(service.AnalysisInput{
		Mode:          service.AnalysisModeDefault,
		SessionID:     ctx.SessionID,
		UserID:        ctx.UserID,
		Message:       ctx.Message,
		ReferenceTime: ctx.ReceivedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("分析消息失败: %v", err)
	}

	if shouldEnterLongChat(analysis) {
		sm.SetState(ctx.SessionID, state.StateLongChat)
	}

	return applyStructuredReply(c, ctx, sm, analysis, false)
}

func (h *EmotionHandler) startAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, emotion, intention string) (*ProcessResult, error) {
	sm.SetState(ctx.SessionID, state.StateLongChat)

	conversation := buildPromptConversation(ctx, sm, BuildLegacySystemPrompt(ctx))

	startedAt := time.Now()
	_, responses, err := aifunction.QueryaiWithChain(conversation)
	metrics.ObserveDuration(
		"bot_ai_request_duration",
		"AI request duration.",
		time.Since(startedAt),
		map[string]string{"kind": "chat", "mode": "start"},
	)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "chat", "mode": "start", "result": "error"},
		)
		utils.Errorw("ai chat failed, sending fallback reply",
			utils.String("request_id", ctx.RequestID),
			utils.String("session_id", ctx.SessionID),
			utils.Int64("user_id", ctx.UserID),
			utils.Err(err),
		)
		fallback, sendErr := sendAIFallbackReply(c, ctx)
		if sendErr != nil {
			return nil, fmt.Errorf("AI chat failed and fallback send failed: %v / %v", err, sendErr)
		}
		return &ProcessResult{
			Handled:   true,
			Emotion:   emotion,
			Intention: intention,
			Reply:     fallback,
		}, nil
	}
	metrics.IncCounter(
		"bot_ai_requests_total",
		"Total AI requests by kind and result.",
		map[string]string{"kind": "chat", "mode": "start", "result": "ok"},
	)

	var deliveredReply string
	for _, response := range responses {
		var sendErr error
		deliveredReply, sendErr = sendContextReply(c, ctx, response)
		if sendErr != nil {
			return nil, fmt.Errorf("发送AI回复失败: %v", sendErr)
		}
	}

	return &ProcessResult{
		Handled:   true,
		Emotion:   emotion,
		Intention: intention,
		Reply:     deliveredReply,
	}, nil
}

// LongChatHandler AI长对话处理器
type LongChatHandler struct{}

func NewLongChatHandler() *LongChatHandler {
	return &LongChatHandler{}
}

func (h *LongChatHandler) CanHandle(ctx MessageContext, sm *state.StateManager) bool {
	return sm.GetState(ctx.SessionID) == state.StateLongChat
}

func (h *LongChatHandler) Handle(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (*ProcessResult, error) {
	analysis, err := service.AnalyzeMessage(service.AnalysisInput{
		Mode:          service.AnalysisModeLongChat,
		SessionID:     ctx.SessionID,
		UserID:        ctx.UserID,
		Message:       ctx.Message,
		ReferenceTime: ctx.ReceivedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("分析消息失败: %v", err)
	}

	sm.SetState(ctx.SessionID, state.StateLongChat)
	return applyStructuredReply(c, ctx, sm, analysis, true)
}

func (h *LongChatHandler) continueAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, emotion, intention string) (*ProcessResult, error) {
	conversation := buildPromptConversation(ctx, sm, BuildLegacySystemPrompt(ctx))

	startedAt := time.Now()
	_, responses, err := aifunction.QueryaiWithChain(conversation)
	metrics.ObserveDuration(
		"bot_ai_request_duration",
		"AI request duration.",
		time.Since(startedAt),
		map[string]string{"kind": "chat", "mode": "continue"},
	)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "chat", "mode": "continue", "result": "error"},
		)
		utils.Errorw("ai conversation failed, sending fallback reply",
			utils.String("request_id", ctx.RequestID),
			utils.String("session_id", ctx.SessionID),
			utils.Int64("user_id", ctx.UserID),
			utils.Err(err),
		)
		fallback, sendErr := sendAIFallbackReply(c, ctx)
		if sendErr != nil {
			return nil, fmt.Errorf("AI conversation failed and fallback send failed: %v / %v", err, sendErr)
		}
		return &ProcessResult{
			Handled:   true,
			Emotion:   emotion,
			Intention: intention,
			Reply:     fallback,
		}, nil
	}
	metrics.IncCounter(
		"bot_ai_requests_total",
		"Total AI requests by kind and result.",
		map[string]string{"kind": "chat", "mode": "continue", "result": "ok"},
	)

	var deliveredReply string
	for _, response := range responses {
		var sendErr error
		deliveredReply, sendErr = sendContextReply(c, ctx, response)
		if sendErr != nil {
			return nil, fmt.Errorf("发送AI回复失败: %v", sendErr)
		}
	}

	return &ProcessResult{
		Handled:   true,
		Emotion:   emotion,
		Intention: intention,
		Reply:     deliveredReply,
	}, nil
}

func (h *LongChatHandler) endAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (string, error) {
	reply := "好吧，那拜拜。"
	if _, err := sendContextReply(c, ctx, reply); err != nil {
		return "", fmt.Errorf("发送结束回复失败: %v", err)
	}

	sm.SetState(ctx.SessionID, state.StateIdle)

	return reply, nil
}

func shouldEnterLongChat(analysis service.MessageAnalysis) bool {
	switch analysis.Intention {
	case "想和对方聊天", "想被对方鼓励", "想和对方倾诉":
		return true
	}
	switch analysis.SupportStrategy {
	case "comfort", "encourage", "continue_chat", "answer_directly":
		return analysis.ReplyMode != service.ReplyModeNoReply
	}
	return false
}

func applyStructuredReply(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, analysis service.MessageAnalysis, longChat bool) (*ProcessResult, error) {
	sm.SetDialogueState(ctx.SessionID, state.DialogueState{
		Emotion:          analysis.Emotion,
		Intention:        analysis.Intention,
		ReplyExpectation: analysis.ReplyExpectation,
		TurnStatus:       analysis.TurnStatus,
		SupportStrategy:  analysis.SupportStrategy,
		Topic:            analysis.Topic,
		UserNeed:         analysis.UserNeed,
		Confidence:       analysis.Confidence,
	})

	if analysis.ReplyMode == service.ReplyModeNoReply {
		if longChat && analysis.WannaBye == "想结束对话" {
			sm.SetState(ctx.SessionID, state.StateIdle)
		}
		return &ProcessResult{
			Handled:   true,
			Replied:   false,
			Emotion:   analysis.Emotion,
			Intention: analysis.Intention,
			ReplyMode: analysis.ReplyMode,
			Reply:     "",
		}, nil
	}

	var (
		reply string
		err   error
	)

	switch analysis.ReplyMode {
	case service.ReplyModeLightAck:
		reply = service.SelectLightAck(ctx.SessionID, ctx.Message, analysis)
		if delivered, sendErr := sendContextReply(c, ctx, reply); sendErr != nil {
			err = sendErr
			return nil, err
		} else {
			reply = delivered
		}
	case service.ReplyModeFullReply:
		reply, err = generateNaturalReply(c, ctx, sm, analysis)
		if err != nil {
			return nil, err
		}
	default:
		fallback, fallbackErr := sendAIFallbackReply(c, ctx)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		reply = fallback
	}

	cleanReply := service.BuildAssistantTranscript(reply)

	if longChat && analysis.WannaBye == "想结束对话" {
		sm.SetState(ctx.SessionID, state.StateIdle)
	}

	return &ProcessResult{
		Handled:   true,
		Replied:   true,
		Emotion:   analysis.Emotion,
		Intention: analysis.Intention,
		ReplyMode: analysis.ReplyMode,
		Reply:     cleanReply,
	}, nil
}

func generateNaturalReply(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, analysis service.MessageAnalysis) (string, error) {
	systemPrompt := buildGenerationSystemPrompt(ctx, analysis)
	conversation := buildPromptConversation(ctx, sm, systemPrompt)

	startedAt := time.Now()
	_, responses, err := aifunction.QueryaiWithChain(conversation)
	metrics.ObserveDuration(
		"bot_ai_request_duration",
		"AI request duration.",
		time.Since(startedAt),
		map[string]string{"kind": "chat", "mode": "generate"},
	)
	if err != nil {
		metrics.IncCounter(
			"bot_ai_requests_total",
			"Total AI requests by kind and result.",
			map[string]string{"kind": "chat", "mode": "generate", "result": "error"},
		)
		utils.Errorw("ai generation failed, sending fallback reply",
			utils.String("request_id", ctx.RequestID),
			utils.String("session_id", ctx.SessionID),
			utils.Int64("user_id", ctx.UserID),
			utils.Err(err),
		)
		fallback, sendErr := sendAIFallbackReply(c, ctx)
		if sendErr != nil {
			return "", fmt.Errorf("ai generation failed and fallback send failed: %v / %v", err, sendErr)
		}
		return fallback, nil
	}
	metrics.IncCounter(
		"bot_ai_requests_total",
		"Total AI requests by kind and result.",
		map[string]string{"kind": "chat", "mode": "generate", "result": "ok"},
	)

	var deliveredReply string
	for _, response := range responses {
		var sendErr error
		deliveredReply, sendErr = sendContextReply(c, ctx, response)
		if sendErr != nil {
			return "", fmt.Errorf("发送AI回复失败: %v", sendErr)
		}
	}

	if len(responses) == 0 {
		return "", fmt.Errorf("ai generation returned empty responses")
	}
	return deliveredReply, nil
}

func buildGenerationSystemPrompt(ctx MessageContext, analysis service.MessageAnalysis) string {
	systemPrompt := BuildLegacySystemPrompt(ctx)
	sections := []string{
		strings.TrimSpace(systemPrompt),
		buildReplyDecisionGuidance(analysis),
		buildReplyOutputContract(analysis),
	}
	return strings.Join(sections, "\n\n")
}

func BuildLegacySystemPrompt(ctx MessageContext) string {
	cfg := config.GetConfig()
	systemPrompt := strings.TrimSpace(cfg.AiPrompt)
	if systemPrompt == "" {
		systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
	}
	sections := []string{
		strings.TrimSpace(systemPrompt),
		"【Legacy Handler Contract】\n旧回复链路必须与 ReAct runtime 保持同一角色身份和最终输出边界。",
		agent.BuildFinalOutputContract(),
	}
	result := strings.Join(sections, "\n\n")
	if cfg.EnableEmotionalMemory || cfg.EnableTimeContext {
		result = service.EnhancePromptWithMemory(ctx.UserID, ctx.SessionID, result, ctx.Message, ctx.ReceivedAt)
	}
	return result
}

func buildReplyDecisionGuidance(analysis service.MessageAnalysis) string {
	lines := []string{
		"【当前回复决策】",
		"你现在处于正式回复阶段，请自然说话，不要输出 JSON，不要解释决策过程。",
		fmt.Sprintf("support_strategy: %s", analysis.SupportStrategy),
		fmt.Sprintf("topic: %s", fallbackString(analysis.Topic, "未知")),
		fmt.Sprintf("user_need: %s", fallbackString(analysis.UserNeed, "被回应")),
		fmt.Sprintf("turn_status: %s", analysis.TurnStatus),
	}

	if strings.TrimSpace(analysis.Intention) != "" {
		lines = append(lines, "intention: "+analysis.Intention)
	}
	if strings.TrimSpace(analysis.Emotion) != "" {
		lines = append(lines, "emotion: "+analysis.Emotion)
	}

	switch analysis.WannaBye {
	case "想结束对话":
		lines = append(lines, "用户更像在收束对话。请自然完成最后一轮回复，可以礼貌告别。")
	default:
		lines = append(lines, "请继续自然推进对话，不要显得像在执行表单。")
	}

	lines = append(lines, "回复要求：保持自然、连贯、像真实聊天，不要重复上下文摘要。")
	return strings.Join(lines, "\n")
}

func buildReplyOutputContract(analysis service.MessageAnalysis) string {
	cfg := config.GetConfig()
	lines := []string{
		"【最终输出要求】",
		"只输出最终要发给用户的话，不要解释规则，不要输出 JSON，不要复述系统指令。",
	}

	if analysis.ReplyExpectation == "high" || analysis.SupportStrategy == "comfort" || analysis.SupportStrategy == "answer_directly" {
		lines = append(lines, "这次回复大概率不止一句短话。")
	}

	if cfg.EnableSpaceSegmentDelimiter {
		lines = append(lines,
			"如果回复只有一句很短的话，可以不分段。",
			"如果回复包含两个及以上语义段，优先使用 $ 作为分段标记；如果你自然输出为空格分段，也允许使用空格隔开各段。",
			"如果回复偏长，必须主动拆成 2 到 4 段，并使用 $ 或空格连接各段。",
			"不要用换行、序号、项目符号代替分段。",
			"每一段都要是自然完整的一句话，不要把一句话硬切碎。",
			"输出示例：第一句$第二句$第三句 或 第一句 第二句 第三句",
			"最后自检：只要不是单句短回应，就必须明确分段后再输出。",
		)
	} else {
		lines = append(lines,
			"如果回复只有一句很短的话，可以不分段。",
			"如果回复包含两个及以上语义段，必须使用 $ 作为分段标记。",
			"如果回复偏长，必须主动拆成 2 到 4 段，并使用 $ 连接。",
			"不要用换行、序号、项目符号代替 $ 分段。",
			"每一段都要是自然完整的一句话，不要把一句话硬切碎。",
			"输出示例：第一句$第二句$第三句",
			"最后自检：只要不是单句短回应，就必须带 $ 再输出。",
		)
	}

	return strings.Join(lines, "\n")
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func buildPromptConversation(ctx MessageContext, sm *state.StateManager, systemPrompt string) []openai.ChatCompletionMessage {
	cfg := config.GetConfig()
	return ensureSystemPrompt(
		selectRecentTurnMessages(sm.GetConversation(ctx.SessionID), cfg.ContextRecentTurns),
		systemPrompt,
	)
}

func selectRecentTurnMessages(conversation []openai.ChatCompletionMessage, recentTurns int) []openai.ChatCompletionMessage {
	if recentTurns <= 0 {
		recentTurns = config.GetConfig().ContextRecentTurns
	}
	if recentTurns <= 0 {
		recentTurns = 8
	}

	start := firstNonSystemMessageIndex(conversation)
	seenUserTurns := 0
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i].Role != openai.ChatMessageRoleUser {
			continue
		}
		seenUserTurns++
		if seenUserTurns == recentTurns {
			start = i
			break
		}
	}

	selected := make([]openai.ChatCompletionMessage, 0, len(conversation)-start)
	for _, msg := range conversation[start:] {
		if msg.Role == openai.ChatMessageRoleSystem {
			continue
		}
		if msg.Role != openai.ChatMessageRoleUser && msg.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" && len(msg.MultiContent) == 0 {
			continue
		}
		selected = append(selected, msg)
	}

	return selected
}

func firstNonSystemMessageIndex(conversation []openai.ChatCompletionMessage) int {
	for i, msg := range conversation {
		if msg.Role != openai.ChatMessageRoleSystem {
			return i
		}
	}
	return len(conversation)
}

// ProcessResult 消息处理结果
type ProcessResult struct {
	Handled         bool              // 是否被处理
	Replied         bool              // 是否真正发送了回复
	MemoryManaged   bool              // 当前路径是否已经显式处理长期记忆写入
	ScheduleManaged bool              // 当前路径是否已经显式处理主动触达计划
	Emotion         string            // 检测到的情感
	Intention       string            // 检测到的意图
	ReplyMode       service.ReplyMode // 回复模式
	Reply           string            // 回复内容
}

// MessageProcessor 消息处理器管理器
type MessageProcessor struct {
	handlers []MessageHandler
}

func NewMessageProcessor() *MessageProcessor {
	return &MessageProcessor{
		handlers: []MessageHandler{
			NewPresetHandler(),
			NewEmotionHandler(),
			NewLongChatHandler(),
		},
	}
}

// Process 处理消息并返回详细结果
func (mp *MessageProcessor) Process(c *websocket.Conn, ctx MessageContext) (*ProcessResult, error) {
	sm := state.GetManager()
	sm.EnsureSession(ctx.SessionID, ctx.UserID, ctx.GroupID, ctx.ChatType)

	if config.GetConfig().EnableOnlyLongChat {
		result, err := mp.handlers[2].Handle(c, ctx, sm)
		if err != nil {
			return &ProcessResult{}, err
		}
		if result == nil {
			return &ProcessResult{}, nil
		}
		return result, nil
	}

	for _, handler := range mp.handlers {
		if !handler.CanHandle(ctx, sm) {
			continue
		}

		result, err := handler.Handle(c, ctx, sm)
		if err != nil {
			return &ProcessResult{}, err
		}
		if result == nil {
			return &ProcessResult{}, nil
		}
		return result, nil
	}

	_, err := sendContextReply(c, ctx, "?")
	return &ProcessResult{
		Handled:   true,
		Replied:   true,
		ReplyMode: service.ReplyModeFullReply,
		Reply:     "?",
	}, err
}

func BuildConversationUserMessage(ctx MessageContext) (openai.ChatCompletionMessage, bool) {
	cfg := config.GetConfig()
	if !cfg.EnableVisionInput {
		return openai.ChatCompletionMessage{
			Role:    "user",
			Content: ctx.Message,
		}, true
	}

	parts := make([]openai.ChatMessagePart, 0, len(ctx.Parts)+1)
	if strings.TrimSpace(ctx.Message) != "" {
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: ctx.Message,
		})
	}

	detail := normalizeVisionImageDetail(cfg.VisionImageDetail)
	for _, part := range ctx.Parts {
		if part.Type != "image" || strings.TrimSpace(part.URL) == "" {
			continue
		}
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    strings.TrimSpace(part.URL),
				Detail: detail,
			},
		})
	}

	if len(parts) == 0 {
		return openai.ChatCompletionMessage{}, false
	}
	if len(parts) == 1 && parts[0].Type == openai.ChatMessagePartTypeText {
		return openai.ChatCompletionMessage{
			Role:    "user",
			Content: parts[0].Text,
		}, true
	}

	return openai.ChatCompletionMessage{
		Role:         "user",
		MultiContent: parts,
	}, true
}

func normalizeVisionImageDetail(raw string) openai.ImageURLDetail {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(openai.ImageURLDetailHigh):
		return openai.ImageURLDetailHigh
	case string(openai.ImageURLDetailLow):
		return openai.ImageURLDetailLow
	default:
		return openai.ImageURLDetailAuto
	}
}

func ensureSystemPrompt(conversation []openai.ChatCompletionMessage, systemPrompt string) []openai.ChatCompletionMessage {
	if strings.TrimSpace(systemPrompt) == "" {
		return append([]openai.ChatCompletionMessage(nil), conversation...)
	}

	if len(conversation) > 0 && conversation[0].Role == "system" {
		updated := append([]openai.ChatCompletionMessage(nil), conversation...)
		updated[0].Content = systemPrompt
		return updated
	}

	updated := make([]openai.ChatCompletionMessage, 0, len(conversation)+1)
	updated = append(updated, openai.ChatCompletionMessage{
		Role:    "system",
		Content: systemPrompt,
	})
	updated = append(updated, conversation...)
	return updated
}
