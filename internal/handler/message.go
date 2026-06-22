package handler

import (
	"fmt"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/memory"
	"project-yume/internal/metrics"
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
		Handled: true,
		Reply:   response,
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
	analysis, err := service.AnalyzeMessage(ctx.Message, service.AnalysisModeDefault)
	if err != nil {
		return nil, fmt.Errorf("分析消息失败: %v", err)
	}

	cfg := config.GetConfig()
	userID := ctx.UserID

	var response string

	switch analysis.Emotion {
	case "开心":
		response = "那很好了。"
	case "生气":
		response = "我做错什么了？"
	case "哲学":
		response = "乐"
	default:
		if analysis.Intention == "想和对方聊天" || analysis.Intention == "想和对方倾诉" {
			sm.SetState(ctx.SessionID, state.StateLongChat)
			return h.startAIChat(c, ctx, sm, analysis.Emotion, analysis.Intention)
		}
		response = "?"
	}

	if cfg.EnableEmotionalMemory {
		response = h.optimizeResponseWithMemory(userID, ctx.Message, analysis.Emotion, analysis.Intention, response)
	}

	if err := service.SendMsg(c, ctx.UserID, response); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Handled:   true,
		Emotion:   analysis.Emotion,
		Intention: analysis.Intention,
		Reply:     response,
	}, nil
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

	if len(conversation) == 0 {
		systemPrompt := cfg.AiPrompt
		if systemPrompt == "" {
			systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
		}

		if cfg.EnableEmotionalMemory || cfg.EnableTimeContext {
			systemPrompt = service.EnhancePromptWithMemory(userID, ctx.SessionID, systemPrompt, ctx.Message, ctx.ReceivedAt)
		}

		utils.Info("【AI对话启动】注入系统 Prompt (长度: %d): %s...", len(systemPrompt), func() string {
			if len(systemPrompt) > 20 {
				return systemPrompt[:20]
			}
			return systemPrompt
		}())

		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: ctx.Message,
	})

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
		Reply:     responses[len(responses)-1],
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
	analysis, err := service.AnalyzeMessage(ctx.Message, service.AnalysisModeLongChat)
	if err != nil {
		return nil, fmt.Errorf("分析消息失败: %v", err)
	}

	if config.GetConfig().EnableOnlyLongChat {
		return h.continueAIChat(c, ctx, sm, analysis.Emotion, analysis.Intention)
	}

	if analysis.WannaBye == "想结束对话" {
		reply, err := h.endAIChat(c, ctx, sm)
		if err != nil {
			return nil, err
		}
		return &ProcessResult{
			Handled:   true,
			Emotion:   analysis.Emotion,
			Intention: analysis.Intention,
			Reply:     reply,
		}, nil
	}

	return h.continueAIChat(c, ctx, sm, analysis.Emotion, analysis.Intention)
}

func (h *LongChatHandler) continueAIChat(c *websocket.Conn, ctx MessageContext, sm *state.StateManager, emotion, intention string) (*ProcessResult, error) {
	cfg := config.GetConfig()
	userID := ctx.UserID

	conversation := sm.GetConversation(ctx.SessionID)

	if len(conversation) == 0 {
		systemPrompt := cfg.AiPrompt
		if systemPrompt == "" {
			systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
		}

		if cfg.EnableEmotionalMemory || cfg.EnableTimeContext {
			systemPrompt = service.EnhancePromptWithMemory(userID, ctx.SessionID, systemPrompt, ctx.Message, ctx.ReceivedAt)
		}

		utils.Info("【AI对话启动】(OnlyLongChat) 注入系统 Prompt (长度: %d): %s...", len(systemPrompt), func() string {
			if len(systemPrompt) > 20 {
				return systemPrompt[:20]
			}
			return systemPrompt
		}())

		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: ctx.Message,
	})

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
		Reply:     responses[len(responses)-1],
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

// ProcessResult 消息处理结果
type ProcessResult struct {
	Handled   bool   // 是否被处理
	Emotion   string // 检测到的情感
	Intention string // 检测到的意图
	Reply     string // 回复内容
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
		Handled: true,
		Reply:   "?",
	}, err
}
