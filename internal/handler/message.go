package handler

import (
	"fmt"
	"strings"
	"time"

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
	Handle(c *websocket.Conn, message, emotion, intention string, sm *state.StateManager) error
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
func (h *PresetHandler) Handle(c *websocket.Conn, message, emotion, intention string, sm *state.StateManager) error {
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

func (h *EmotionHandler) Handle(c *websocket.Conn, message, emotion, intention string, sm *state.StateManager) error {
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

	return service.SendMsg(c, cfg.TargetId, response)
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
	optimizedResponse := h.adjustResponseByPattern(originalResponse, pattern, emotion)

	return optimizedResponse
}

// adjustResponseByPattern 根据对话模式调整回复
func (h *EmotionHandler) adjustResponseByPattern(response, pattern, emotion string) string {
	switch pattern {
	case "需要关怀":
		return h.addComfortTone(response, emotion)
	case "积极活跃":
		return h.addCheerfulTone(response, emotion)
	case "情绪波动":
		return h.addCalmTone(response, emotion)
	case "新用户":
		return h.addWelcomeTone(response, emotion)
	default:
		return response
	}
}

// addComfortTone 添加关怀语调
func (h *EmotionHandler) addComfortTone(response, emotion string) string {
	comfortPrefixes := []string{"", "别担心，", "我理解你，", "没关系的，"}
	comfortSuffixes := []string{"", "，我会陪着你", "，一切都会好起来的", "，你不是一个人"}

	if emotion == "难过" {
		return "我一直都在这里陪着你" + comfortSuffixes[1]
	}
	if emotion == "生气" {
		return comfortPrefixes[1] + response + comfortSuffixes[0]
	}

	return comfortPrefixes[0] + response + comfortSuffixes[0]
}

// addCheerfulTone 添加活泼语调
func (h *EmotionHandler) addCheerfulTone(response, emotion string) string {
	cheerfulPrefixes := []string{"", "哈哈，", "嘿嘿，", "哇，"}
	cheerfulSuffixes := []string{"", "！", "~", "呢！"}

	if emotion == "开心" {
		return cheerfulPrefixes[1] + "你的好心情也感染到我了" + cheerfulSuffixes[1]
	}
	if emotion == "中性" {
		return cheerfulPrefixes[0] + response + cheerfulSuffixes[1]
	}

	return response + cheerfulSuffixes[0]
}

// addCalmTone 添加平静语调
func (h *EmotionHandler) addCalmTone(response, emotion string) string {
	calmPrefixes := []string{"", "嗯，", "好的，", "我明白，"}
	calmSuffixes := []string{"", "，我们慢慢聊", "，不着急", "，慢慢来"}

	if strings.Contains(response, "?") || strings.Contains(response, "？") {
		return calmPrefixes[3] + response + calmSuffixes[1]
	}

	return calmPrefixes[0] + response + calmSuffixes[0]
}

// addWelcomeTone 添加欢迎语调
func (h *EmotionHandler) addWelcomeTone(response, emotion string) string {
	welcomePrefixes := []string{"", "欢迎！", "你好呀，", "很高兴认识你，"}
	welcomeSuffixes := []string{"", "，有什么想聊的吗？", "，我们可以慢慢了解", ""}

	if emotion == "中性" && response == "?" {
		return welcomePrefixes[2] + "有什么想聊的吗？"
	}

	return response + welcomeSuffixes[0]
}

// enhancePromptWithMemory 基于情感记忆增强AI提示词
func (h *EmotionHandler) enhancePromptWithMemory(userID int64, originalPrompt string) string {
	memoryManager := memory.GetManager()

	// 获取用户对话模式
	pattern := memoryManager.GetConversationPattern(userID)

	// 获取最近情感状态
	recentEmotions := memoryManager.GetRecentEmotions(userID, 5)

	// 构建情感记忆上下文
	emotionalContext := h.buildEmotionalContext(pattern, recentEmotions)

	// 增强提示词
	enhancedPrompt := originalPrompt + "\n\n" + emotionalContext

	return enhancedPrompt
}

// buildEmotionalContext 构建情感上下文
func (h *EmotionHandler) buildEmotionalContext(pattern string, recentEmotions []string) string {
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

func (h *EmotionHandler) startAIChat(c *websocket.Conn, message string, sm *state.StateManager) error {
	cfg := config.GetConfig()
	userID := cfg.TargetId

	// 设置长对话标志
	// sm.SetFlag(state.FlagLongChain, true)
	sm.SetState(state.StateLongChat)

	// 获取当前对话历史
	conversation := sm.GetConversation()

	// 如果是新对话，添加系统提示
	if len(conversation) == 0 {
		systemPrompt := cfg.AiPrompt
		if systemPrompt == "" {
			systemPrompt = "你是一个温暖、友善的聊天伙伴。请用自然、亲切的语气与用户对话，回复要简短而有趣。"
		}

		// 情感记忆增强系统提示词
		if cfg.EnableEmotionalMemory {
			systemPrompt = h.enhancePromptWithMemory(userID, systemPrompt)
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

func (h *LongChatHandler) Handle(c *websocket.Conn, message, emotion, intention string, sm *state.StateManager) error {
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
	cfg := config.GetConfig()
	userID := cfg.TargetId

	// 获取当前对话历史
	conversation := sm.GetConversation()

	// 添加用户消息
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    "user",
		Content: message,
	})

	// 情感记忆增强：动态调整系统提示词
	if cfg.EnableEmotionalMemory && len(conversation) > 1 {
		conversation = h.updateSystemPromptWithMemory(userID, conversation)
	}

	// 生成日志文件路径
	filepath := "./public/aichatlog/longchain/log_" + time.Now().Format("06-01-02") + ".txt"

	// 调用AI进行对话
	newConversation, responses, err := aifunction.QueryaiWithChain(conversation, filepath)
	if err != nil {
		return fmt.Errorf("AI对话失败: %v", err)
	}

	// 发送AI回复
	for _, response := range responses {
		err := service.SendMsg(c, cfg.TargetId, response)
		if err != nil {
			return fmt.Errorf("发送AI回复失败: %v", err)
		}
	}

	// 更新对话历史
	sm.SetConversation(newConversation)

	return nil
}

// updateSystemPromptWithMemory 基于情感记忆更新系统提示词
func (h *LongChatHandler) updateSystemPromptWithMemory(userID int64, conversation []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	memoryManager := memory.GetManager()

	// 获取用户对话模式
	pattern := memoryManager.GetConversationPattern(userID)

	// 获取最近情感状态
	recentEmotions := memoryManager.GetRecentEmotions(userID, 3)

	// 构建情感上下文更新
	emotionalUpdate := h.buildEmotionalUpdate(pattern, recentEmotions)

	// 如果有情感更新信息，添加到对话中
	if emotionalUpdate != "" {
		// 在用户消息前插入情感上下文更新
		updatedConversation := make([]openai.ChatCompletionMessage, 0, len(conversation)+1)

		// 复制系统消息
		if len(conversation) > 0 && conversation[0].Role == "system" {
			updatedConversation = append(updatedConversation, conversation[0])
		}

		// 添加情感上下文更新
		updatedConversation = append(updatedConversation, openai.ChatCompletionMessage{
			Role:    "system",
			Content: emotionalUpdate,
		})

		// 复制其余消息
		startIndex := 1
		if len(conversation) > 0 && conversation[0].Role == "system" {
			startIndex = 1
		} else {
			startIndex = 0
		}

		for i := startIndex; i < len(conversation); i++ {
			updatedConversation = append(updatedConversation, conversation[i])
		}

		return updatedConversation
	}

	return conversation
}

// buildEmotionalUpdate 构建情感更新信息
func (h *LongChatHandler) buildEmotionalUpdate(pattern string, recentEmotions []string) string {
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

	for _, handler := range mp.handlers {
		if handler.CanHandle(message, sm) {
			emotion, err := analyzeEmotion(message)
			if err != nil {
				return result, fmt.Errorf("分析情感失败: %v", err)
			}
			intention, err := analyzeIntention(message)
			if err != nil {
				return result, fmt.Errorf("分析意图失败: %v", err)
			}
			result.Emotion = emotion
			result.Intention = intention

			err = handler.Handle(c, message, emotion, intention, sm)
			if err != nil {
				return result, err
			}

			result.Handled = true
			return result, nil
		}
	}

	// 如果没有处理器能处理，返回默认回复
	err := service.SendMsg(c, config.GetConfig().TargetId, "?")
	result.Reply = "?"
	result.Handled = true

	return result, err
}

// 公共方法
// analyzeEmotion 分析情感
func analyzeEmotion(message string) (string, error) {
	prompt := "请帮我分析下这段话的情感，并在下面六个选项中选择：开心，生气，中性，哲学，敷衍，难过， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容，也不需要添加分号"
	return aifunction.Queryai(prompt, message)
}

// analyzeIntention 分析意图
func analyzeIntention(message string) (string, error) {
	prompt := "请帮我分析下这段话的意图，并在下面六个选项中选择：想和对方聊天，想被对方鼓励，想和对方倾诉，安慰对方，鼓励对方，和对方道歉 并只回复选项，例如：\"user: 能陪我会儿吗\" resp: \"想和对方倾诉\", 不需要回答多余的内容，也不需要添加分号"
	return aifunction.Queryai(prompt, message)
}
