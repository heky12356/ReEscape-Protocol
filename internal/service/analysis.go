package service

import (
	"strings"

	"project-yume/internal/aifunction"
	"project-yume/internal/memory"
	"project-yume/internal/utils"

	"github.com/sashabaranov/go-openai"
)

// AnalyzeEmotion 分析情感
func AnalyzeEmotion(message string) (string, error) {
	prompt := "请帮我分析下这段话的情感，并在下面六个选项中选择：开心，生气，中性，哲学，敷衍，难过， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容，也不需要添加分号"
	result, err := aifunction.Queryai(prompt, message)
	if err != nil {
		utils.Warn("AnalyzeEmotion fallback to 中性 due to AI error: %v", err)
		return "中性", nil
	}
	return result, nil
}

// AnalyzeIntention 分析意图
func AnalyzeIntention(message string) (string, error) {
	prompt := "请帮我分析下这段话的意图，并在下面六个选项中选择：想和对方聊天，想被对方鼓励，想和对方倾诉，安慰对方，鼓励对方，和对方道歉 并只回复选项，例如：\"user: 能陪我会儿吗\" resp: \"想和对方倾诉\", 不需要回答多余的内容，也不需要添加分号"
	result, err := aifunction.Queryai(prompt, message)
	if err != nil {
		utils.Warn("AnalyzeIntention fallback to 想和对方聊天 due to AI error: %v", err)
		return "想和对方聊天", nil
	}
	return result, nil
}

// AnalyzeWannaBye 分析是否想结束对话
func AnalyzeWannaBye(message string) (string, error) {
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
	result, err := aifunction.Queryai(prompt, message)
	if err != nil {
		utils.Warn("AnalyzeWannaBye fallback to 想继续 due to AI error: %v", err)
		return "想继续", nil
	}
	return result, nil
}

// EnhancePromptWithMemory 基于情感记忆增强AI提示词
func EnhancePromptWithMemory(userID int64, originalPrompt string) string {
	memoryManager := memory.GetManager()

	// 获取用户对话模式
	pattern := memoryManager.GetConversationPattern(userID)

	// 获取最近情感状态
	recentEmotions := memoryManager.GetRecentEmotions(userID, 5)

	// 构建情感记忆上下文
	emotionalContext := BuildEmotionalContext(pattern, recentEmotions)

	// 增强提示词
	enhancedPrompt := originalPrompt + "\n\n" + emotionalContext

	return enhancedPrompt
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

// UpdateSystemPromptWithMemory 基于情感记忆更新系统提示词
func UpdateSystemPromptWithMemory(userID int64, conversation []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	memoryManager := memory.GetManager()

	// 获取用户对话模式
	pattern := memoryManager.GetConversationPattern(userID)

	// 获取最近情感状态
	recentEmotions := memoryManager.GetRecentEmotions(userID, 3)

	// 构建情感上下文更新
	emotionalUpdate := BuildEmotionalUpdate(pattern, recentEmotions)

	// 如果有情感更新信息，更新对话中的情感状态
	if emotionalUpdate != "" {
		// 查找并替换现有的情感状态更新消息
		for i, msg := range conversation {
			if msg.Role == "system" && strings.Contains(msg.Content, "【情感状态更新】") {
				// 找到现有的情感状态更新，直接替换
				conversation[i].Content = emotionalUpdate
				return conversation
			}
		}

		// 如果没有找到现有的情感状态更新，则在第一个系统消息后插入新的
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
