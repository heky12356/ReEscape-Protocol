package handler

import (
	"fmt"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/memory"
	"project-yume/internal/service"
	"project-yume/internal/state"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	CanHandle(message string, sm *state.StateManager) bool
	Handle(c *websocket.Conn, message string, sm *state.StateManager) (string, error)
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
		},
	}
}

// CanHandle 检查是否可以处理消息
func (h *PresetHandler) CanHandle(message string, sm *state.StateManager) bool {
	// 如果不是空闲状态就不处理，让其他处理器处理
	if sm.GetState() != state.StateIdle {
		return false
	}
	_, exists := h.responses[message]
	return exists
}

// Handle 处理消息
func (h *PresetHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) (string, error) {
	response := h.responses[message]

	// 特殊处理
	if message == "在忙呢" {
		sm.SetState(state.StateBusy)
	}
	if message == "能陪我聊聊吗" {
		sm.SetState(state.StateLongChat)
	}

	return response, service.SendMsg(c, config.GetConfig().TargetId, response)
}

// EmotionHandler 情感分析处理器
type EmotionHandler struct{}

func NewEmotionHandler() *EmotionHandler {
	return &EmotionHandler{}
}

func (h *EmotionHandler) CanHandle(message string, sm *state.StateManager) bool {
	return sm.GetState() == state.StateIdle
}

func (h *EmotionHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) (string, error) {
	// 在这里进行 AI 分析，只有真正需要处理时才调用
	emotion, err := service.AnalyzeEmotion(message)
	if err != nil {
		return "", fmt.Errorf("分析情感失败: %v", err)
	}
	intention, err := service.AnalyzeIntention(message)
	if err != nil {
		return "", fmt.Errorf("分析意图失败: %v", err)
	}

	cfg := config.GetConfig()
	userID := cfg.TargetId

	var response string

	// 基础情感回复
	switch emotion {
	case "开心":
		response = "那很好了。"
	case "生气":
		response = "我做错什么了？"
	case "哲学":
		response = "乐"
	default:
		if intention == "想和对方聊天" || intention == "想和对方倾诉" {
			sm.SetState(state.StateLongChat)
			return h.startAIChat(c, message, sm)
		}
		response = "?"
	}

	// 情感记忆优化层
	if cfg.EnableEmotionalMemory {
		response = h.optimizeResponseWithMemory(userID, message, emotion, intention, response)
	}

	return response, service.SendMsg(c, cfg.TargetId, response)
}

// optimizeResponseWithMemory 基于情感记忆优化回复
func (h *EmotionHandler) optimizeResponseWithMemory(userID int64, message, emotion, intention, originalResponse string) string {
	memoryManager := memory.GetManager()

	// 获取用户的对话模式
	pattern := memoryManager.GetConversationPattern(userID)

	// 获取情感记忆建议的回复
	suggestedResponse := memoryManager.SuggestResponse(userID, message, emotion)

	// 如果有建议回复且不为空，优先使用
	if suggestedResponse != "" {
		return suggestedResponse
	}

	// 根据对话模式优化原始回复
	return service.AdjustResponseByPattern(originalResponse, pattern, emotion)
}

func (h *EmotionHandler) startAIChat(c *websocket.Conn, message string, sm *state.StateManager) (string, error) {
	cfg := config.GetConfig()
	userID := cfg.TargetId

	// 设置长对话标志
	// sm.SetFlag(state.FlagLongChain, true)
	sm.SetState(state.StateLongChat)

	// 获取当前对话历史
	conversation := sm.GetConversation(userID)

	// 如果是新对话，添加系统提示
	if len(conversation) == 0 {
		systemPrompt := cfg.AiPrompt
		if systemPrompt == "" {
			systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
		}

		// 情感记忆增强系统提示词
		if cfg.EnableEmotionalMemory {
			systemPrompt = service.EnhancePromptWithMemory(userID, systemPrompt)
		}

		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// 添加用户消息到对话历史
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: message,
	})

	// 调用AI进行对话
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation)
	if err != nil {
		return "", fmt.Errorf("AI聊天失败: %v", err)
	}

	// 发送AI回复
	for _, response := range responses {
		err := service.SendMsg(c, config.GetConfig().TargetId, response)
		if err != nil {
			return "", fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	// 更新对话历史
	sm.SetConversation(userID, newConversation)

	return responses[len(responses)-1], nil
}

// LongChatHandler AI长对话处理器
type LongChatHandler struct{}

func NewLongChatHandler() *LongChatHandler {
	return &LongChatHandler{}
}

func (h *LongChatHandler) CanHandle(message string, sm *state.StateManager) bool {
	return sm.GetState() == state.StateLongChat
}

func (h *LongChatHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) (string, error) {
	if config.GetConfig().EnableOnlyLongChat {
		return h.continueAIChat(c, message, sm)
	}
	// 检查是否要结束对话
	wannaBye, err := service.AnalyzeWannaBye(message)
	if err != nil {
		return "", fmt.Errorf("分析是否要结束对话失败: %v", err)
	}
	if wannaBye == "想结束对话" {
		return "", h.endAIChat(c, sm)
	}

	// 继续AI对话
	return h.continueAIChat(c, message, sm)
}

func (h *LongChatHandler) continueAIChat(c *websocket.Conn, message string, sm *state.StateManager) (string, error) {
	cfg := config.GetConfig()
	userID := cfg.TargetId

	// 获取当前对话历史
	conversation := sm.GetConversation(userID)

	// 添加用户消息
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: message,
	})

	// 情感记忆增强：动态调整系统提示词
	if cfg.EnableEmotionalMemory && len(conversation) > 1 {
		conversation = service.UpdateSystemPromptWithMemory(userID, conversation)
	}

	// 调用AI进行对话
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation)
	if err != nil {
		return "", fmt.Errorf("AI对话失败: %v", err)
	}

	// 发送AI回复
	for _, response := range responses {
		err := service.SendMsg(c, cfg.TargetId, response)
		if err != nil {
			return "", fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	// 更新对话历史
	sm.SetConversation(userID, newConversation)

	return responses[len(responses)-1], nil
}

func (h *LongChatHandler) endAIChat(c *websocket.Conn, sm *state.StateManager) error {
	cfg := config.GetConfig()
	userID := cfg.TargetId

	// 发送结束回复
	err := service.SendMsg(c, userID, "好吧，那拜拜。")
	if err != nil {
		return fmt.Errorf("发送结束回复失败: %v", err)
	}

	// 重置状态
	sm.SetState(state.StateIdle)
	// 清空历史消息
	// sm.ClearConversation(userID)

	return nil
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
func (mp *MessageProcessor) Process(c *websocket.Conn, message string) (*ProcessResult, error) {
	sm := state.GetManager()
	result := &ProcessResult{
		Handled: false,
	}

	if config.GetConfig().EnableOnlyLongChat {
		// OnlyLongChat 模式下，直接交给 LongChatHandler 处理
		// emotion 和 intention 分析可以在 Handler 内部做，或者这里简化处理
		// 既然是 OnlyLongChat，我们假设都想聊
		result.Intention = "想和对方聊天"
		// 注意：Interface 已经变了，不再传递 emotion/intention
		reply, err := mp.handlers[2].Handle(c, message, sm)
		if err != nil {
			return result, err
		}
		result.Handled = true
		result.Reply = reply
		return result, nil
	}

	for _, handler := range mp.handlers {
		if handler.CanHandle(message, sm) {
			// 直接调用 Handle，无需预先分析
			reply, err := handler.Handle(c, message, sm)
			if err != nil {
				return result, err
			}

			result.Handled = true
			result.Reply = reply
			// 注意：result.Emotion 和 result.Intention 现在可能为空，因为分析下放了
			// 如果需要记录，得从 Handler 返回值里拿，但现在 Interface只返回 string, error
			// 这是一个 Trade-off。通常 ProcessResult 主要用于日志。
			return result, nil
		}
	}

	// 如果没有处理器能处理，返回默认回复
	err := service.SendMsg(c, config.GetConfig().TargetId, "?")
	result.Reply = "?"
	result.Handled = true

	return result, err
}
