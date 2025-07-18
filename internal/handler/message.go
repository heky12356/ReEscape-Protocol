package handler

import (
	"fmt"
	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/service"
	"project-yume/internal/state"

	"github.com/gorilla/websocket"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	CanHandle(message string, sm *state.StateManager) bool
	Handle(c *websocket.Conn, message string, sm *state.StateManager) error
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

func (h *PresetHandler) CanHandle(message string, sm *state.StateManager) bool {
	_, exists := h.responses[message]
	return exists
}

func (h *PresetHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) error {
	response := h.responses[message]
	
	// 特殊处理
	if message == "在忙呢" {
		sm.SetFlag("replied", true)
	}
	if message == "能陪我聊聊吗" {
		sm.SetState(state.StateLongChat)
	}
	
	return service.SendMsg(c, config.GetConfig().TargetId, response)
}

// EmotionHandler 情感分析处理器
type EmotionHandler struct{}

func NewEmotionHandler() *EmotionHandler {
	return &EmotionHandler{}
}

func (h *EmotionHandler) CanHandle(message string, sm *state.StateManager) bool {
	return sm.GetState() == state.StateIdle
}

func (h *EmotionHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) error {
	emotion, err := h.analyzeEmotion(message)
	if err != nil {
		return err
	}
	
	var response string
	switch emotion {
	case "开心":
		response = "那很好了。"
	case "生气":
		response = "我做错什么了？"
	case "哲学":
		response = "乐"
	default:
		// 进一步分析意图
		intention, err := h.analyzeIntention(message)
		if err != nil {
			return err
		}
		
		if intention == "想和对方聊天" || intention == "想和对方倾诉" {
			sm.SetState(state.StateLongChat)
			return h.startAIChat(c, message, sm)
		}
		response = "?"
	}
	
	return service.SendMsg(c, config.GetConfig().TargetId, response)
}

func (h *EmotionHandler) analyzeEmotion(message string) (string, error) {
	prompt := "请帮我分析下这段话的情感，并在下面六个选项中选择：开心，生气，中性，哲学，敷衍，难过， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容，也不需要添加分号"
	return aifunction.Queryai(prompt, message)
}

func (h *EmotionHandler) analyzeIntention(message string) (string, error) {
	prompt := "请帮我分析下这段话的意图，并在下面六个选项中选择：想和对方聊天，想被对方鼓励，想和对方倾诉，安慰对方，鼓励对方，和对方道歉 并只回复选项，例如：\"user: 能陪我会儿吗\" resp: \"想和对方倾诉\", 不需要回答多余的内容，也不需要添加分号"
	return aifunction.Queryai(prompt, message)
}

func (h *EmotionHandler) startAIChat(c *websocket.Conn, message string, sm *state.StateManager) error {
	// 实现AI聊天逻辑
	return fmt.Errorf("AI chat not implemented yet")
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
		},
	}
}

func (mp *MessageProcessor) Process(c *websocket.Conn, message string) error {
	sm := state.GetManager()
	
	for _, handler := range mp.handlers {
		if handler.CanHandle(message, sm) {
			return handler.Handle(c, message, sm)
		}
	}
	
	// 默认回复
	return service.SendMsg(c, config.GetConfig().TargetId, "?")
}