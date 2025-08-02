package handler

import (
	"fmt"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/service"
	"project-yume/internal/state"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
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
func (h *PresetHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) error {
	response := h.responses[message]

	// 特殊处理
	if message == "在忙呢" {
		sm.SetState(state.StateBusy)
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
		return fmt.Errorf("情感分析失败: %v", err)
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
	// 设置长对话标志
	// sm.SetFlag(state.FlagLongChain, true)
	sm.SetState(state.StateLongChat)

	// 获取当前对话历史
	conversation := sm.GetConversation()

	// 如果是新对话，添加系统提示
	if len(conversation) == 0 {
		systemPrompt := config.GetConfig().AiPrompt
		if systemPrompt == "" {
			systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
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

	// 生成日志文件路径
	filepath := "./public/aichatlog/longchain/log_" + time.Now().Format("06-01-02") + ".txt"

	// 调用AI进行对话
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation, filepath)
	if err != nil {
		return fmt.Errorf("AI聊天失败: %v", err)
	}

	// 发送AI回复
	for _, response := range responses {
		err := service.SendMsg(c, config.GetConfig().TargetId, response)
		if err != nil {
			return fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	// 更新对话历史
	sm.SetConversation(newConversation)

	return nil
}

// LongChatHandler AI长对话处理器
type LongChatHandler struct{}

func NewLongChatHandler() *LongChatHandler {
	return &LongChatHandler{}
}

func (h *LongChatHandler) CanHandle(message string, sm *state.StateManager) bool {
	return sm.GetState() == state.StateLongChat
}

func (h *LongChatHandler) Handle(c *websocket.Conn, message string, sm *state.StateManager) error {
	// 检查是否要结束对话
	wannaBye, err := h.analyzeWannaBye(message)
	if err != nil {
		return fmt.Errorf("分析是否要结束对话失败: %v", err)
	}
	if wannaBye == "想结束对话" {
		return h.endAIChat(c, sm)
	}

	// 继续AI对话
	return h.continueAIChat(c, message, sm)
}

func (h *LongChatHandler) continueAIChat(c *websocket.Conn, message string, sm *state.StateManager) error {
	// 获取当前对话历史
	conversation := sm.GetConversation()

	// 添加用户消息
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: message,
	})

	// 生成日志文件路径
	filepath := "./public/aichatlog/longchain/log_" + time.Now().Format("06-01-02") + ".txt"

	// 调用AI进行对话
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation, filepath)
	if err != nil {
		return fmt.Errorf("AI对话失败: %v", err)
	}

	// 发送AI回复
	for _, response := range responses {
		err := service.SendMsg(c, config.GetConfig().TargetId, response)
		if err != nil {
			return fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	// 更新对话历史
	sm.SetConversation(newConversation)

	return nil
}

func (h *LongChatHandler) endAIChat(c *websocket.Conn, sm *state.StateManager) error {
	// 发送结束回复
	err := service.SendMsg(c, config.GetConfig().TargetId, "好吧，那拜拜。")
	if err != nil {
		return fmt.Errorf("发送结束回复失败: %v", err)
	}

	// 重置状态
	sm.SetState(state.StateIdle)
	// 清空历史消息
	// sm.ClearConversation()

	return nil
}

func (h *LongChatHandler) analyzeWannaBye(message string) (string, error) {
	prompt := `
	你是一个聊天意图分析助手。当前用户正在与AI进行长对话，请分析用户的这句话是否想要结束当前对话。

	判断标准：
	【想结束对话】的信号：
	- 明确的告别词：再见、拜拜、88、bye、晚安、睡了等
	- 表达要离开：我走了、我去忙了、先这样吧、不聊了等  
	- 礼貌性结束：谢谢你、辛苦了、今天就到这里等
	- 表达疲惫：累了、困了、不想说了等

	【想继续】的信号：
	- 提出新话题：对了、话说、还有等
	- 表达疑惑但想了解：什么意思、为什么、怎么回事等
	- 简单回应：哈哈、嗯、好的、是的等
	- 暂停性词语：等等、稍等、让我想想等
	- 单个字符、表情符号、语气词等

	注意：当意图不明确时，倾向于判断为"想继续"，避免误结束有价值的对话。

	请在以下两个选项中选择：想继续，想结束对话

	示例：
	"哈哈哈" → "想继续"
	"拜拜啦" → "想结束对话"  
	"等等" → "想继续"
	"我去吃饭了" → "想结束对话"
	"啊？" → "想继续"
	"谢谢你今天陪我聊天" → "想结束对话"
	"对了还有个问题" → "想继续"
	"困了要睡觉了" → "想结束对话"

	只回复选项，不需要其他内容。
	`
	return aifunction.Queryai(prompt, message)
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
