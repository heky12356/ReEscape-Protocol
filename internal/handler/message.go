package handler

import (
	"fmt"
	"strings"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/memory"
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
	RequestID    string
	SessionID    string
	UserID       int64
	GroupID      int64
	ChatType     int
	MessageID    int64
	MessageIDs   []int64
	RawSegments  []string
	Parts        []model.MessagePart
	Aggregated   bool
	SegmentCount int
	RawMessage   string
	Message      string
	ReceivedAt   time.Time
	StartedAt    time.Time
	EndedAt      time.Time
	DropReason   string
}

func sendAIFallbackReply(c *websocket.Conn, userID int64) (string, error) {
	if err := service.SendMsg(c, userID, aiFallbackReply); err != nil {
		return "", err
	}
	return aiFallbackReply, nil
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

	if err := service.SendMsg(c, ctx.UserID, response); err != nil {
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
		Conversation:  sm.GetConversation(ctx.SessionID),
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

// optimizeResponseWithMemory 基于情感记忆优化回复
func (h *EmotionHandler) optimizeResponseWithMemory(userID int64, message, emotion, intention, originalResponse string) string {
	memoryManager := memory.GetManager()

	pattern := memoryManager.GetConversationPattern(userID)
	suggestedResponse := memoryManager.SuggestResponse(userID, message, emotion)

	if suggestedResponse != "" {
		return suggestedResponse
	}

	return service.AdjustResponseByPattern(originalResponse, pattern, emotion)
}

func (h *EmotionHandler) startAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, emotion, intention string) (*ProcessResult, error) {
	cfg := config.GetConfig()
	userID := ctx.UserID

	sm.SetState(ctx.SessionID, state.StateLongChat)

	conversation := sm.GetConversation(ctx.SessionID)
	systemPrompt := cfg.AiPrompt
	if systemPrompt == "" {
		systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
	}
	if cfg.EnableEmotionalMemory || cfg.EnableTimeContext {
		systemPrompt = service.EnhancePromptWithMemory(userID, ctx.SessionID, systemPrompt, ctx.Message, ctx.ReceivedAt)
	}
	conversation = ensureSystemPrompt(conversation, systemPrompt)

	startedAt := time.Now()
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation)
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
		fallback, sendErr := sendAIFallbackReply(c, ctx.UserID)
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

	for _, response := range responses {
		if err := service.SendMsg(c, ctx.UserID, response); err != nil {
			return nil, fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	sm.SetConversation(ctx.SessionID, newConversation)

	return &ProcessResult{
		Handled:   true,
		Emotion:   emotion,
		Intention: intention,
		Reply:     service.StripReplyDirectives(responses[len(responses)-1]),
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
		Conversation:  sm.GetConversation(ctx.SessionID),
		ReferenceTime: ctx.ReceivedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("分析消息失败: %v", err)
	}

	sm.SetState(ctx.SessionID, state.StateLongChat)
	return applyStructuredReply(c, ctx, sm, analysis, true)
}

func (h *LongChatHandler) continueAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, emotion, intention string) (*ProcessResult, error) {
	cfg := config.GetConfig()
	userID := ctx.UserID

	conversation := sm.GetConversation(ctx.SessionID)
	systemPrompt := cfg.AiPrompt
	if systemPrompt == "" {
		systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
	}
	if cfg.EnableEmotionalMemory || cfg.EnableTimeContext {
		systemPrompt = service.EnhancePromptWithMemory(userID, ctx.SessionID, systemPrompt, ctx.Message, ctx.ReceivedAt)
	}
	conversation = ensureSystemPrompt(conversation, systemPrompt)

	if (cfg.EnableEmotionalMemory || cfg.EnableTimeContext) && len(conversation) > 1 {
		conversation = service.UpdateSystemPromptWithMemory(userID, ctx.SessionID, ctx.Message, ctx.ReceivedAt, conversation)
	}

	startedAt := time.Now()
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation)
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
		fallback, sendErr := sendAIFallbackReply(c, ctx.UserID)
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

	for _, response := range responses {
		if err := service.SendMsg(c, ctx.UserID, response); err != nil {
			return nil, fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	sm.SetConversation(ctx.SessionID, newConversation)

	return &ProcessResult{
		Handled:   true,
		Emotion:   emotion,
		Intention: intention,
		Reply:     service.StripReplyDirectives(responses[len(responses)-1]),
	}, nil
}

func (h *LongChatHandler) endAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager) (string, error) {
	reply := "好吧，那拜拜。"
	if err := service.SendMsg(c, ctx.UserID, reply); err != nil {
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

	reply := analysis.VisibleReply
	if reply == "" {
		fallback, err := sendAIFallbackReply(c, ctx.UserID)
		if err != nil {
			return nil, err
		}
		reply = fallback
	} else {
		if err := service.SendMsg(c, ctx.UserID, reply); err != nil {
			return nil, err
		}
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

// ProcessResult 消息处理结果
type ProcessResult struct {
	Handled   bool              // 是否被处理
	Replied   bool              // 是否真正发送了回复
	Emotion   string            // 检测到的情感
	Intention string            // 检测到的意图
	ReplyMode service.ReplyMode // 回复模式
	Reply     string            // 回复内容
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

	err := service.SendMsg(c, ctx.UserID, "?")
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
