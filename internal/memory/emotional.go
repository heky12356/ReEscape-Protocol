package memory

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"project-yume/internal/utils"
)

// EmotionalMemory 情感记忆
type EmotionalMemory struct {
	UserID       int64         `json:"user_id"`
	Interactions []Interaction `json:"interactions"`
	Preferences  Preferences   `json:"preferences"`
	LastSeen     time.Time     `json:"last_seen"`
	mu           sync.RWMutex
}

// Preferences 偏好
type Preferences struct {
	EmotionCount   map[string]int `json:"emotion_count"`
	IntentionCount map[string]int `json:"intention_count"`
}

// Interaction 交互记录
type Interaction struct {
	Timestamp time.Time `json:"timestamp"`
	UserMsg   string    `json:"user_msg"`
	BotReply  string    `json:"bot_reply"`
	Emotion   string    `json:"emotion"`
	Intention string    `json:"intention"`
	Context   string    `json:"context"` // 当时的对话上下文
}

// MemoryManager 记忆管理器
type MemoryManager struct {
	memories map[int64]*EmotionalMemory
	mu       sync.RWMutex
	filePath string
}

var manager *MemoryManager

func init() {
	manager = &MemoryManager{
		memories: make(map[int64]*EmotionalMemory),
		filePath: "./public/memory/emotional_memory.json",
	}
	manager.loadFromFile()
}

func GetManager() *MemoryManager {
	return manager
}

// RecordInteraction 记录交互
func (mm *MemoryManager) RecordInteraction(userID int64, userMsg, botReply, emotion, intention string) {
	if emotion == "" || intention == "" {
		utils.Warn("skip RecordInteraction due to empty emotion/intention: emotion=%q intention=%q", emotion, intention)
		return
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.memories[userID] == nil {
		mm.memories[userID] = &EmotionalMemory{
			UserID:       userID,
			Interactions: make([]Interaction, 0),
			Preferences: Preferences{
				EmotionCount:   make(map[string]int),
				IntentionCount: make(map[string]int),
			},
			LastSeen: time.Now(),
		}
	}

	memory := mm.memories[userID]
	memory.mu.Lock()
	defer memory.mu.Unlock()

	interaction := Interaction{
		Timestamp: time.Now(),
		UserMsg:   userMsg,
		BotReply:  botReply,
		Emotion:   emotion,
		Intention: intention,
		Context:   mm.generateContext(memory),
	}

	memory.Interactions = append(memory.Interactions, interaction)
	memory.LastSeen = time.Now()

	// 保持最近100条记录
	if len(memory.Interactions) > 100 {
		memory.Interactions = memory.Interactions[len(memory.Interactions)-100:]
	}

	// 更新偏好
	mm.updatePreferences(memory, emotion, intention)

	// 异步保存到文件
	go mm.saveToFile()
}

// GetRecentEmotions 获取最近的情感状态
func (mm *MemoryManager) GetRecentEmotions(userID int64, count int) []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	memory := mm.memories[userID]
	if memory == nil {
		return []string{}
	}

	memory.mu.RLock()
	defer memory.mu.RUnlock()

	emotions := make([]string, 0)
	start := len(memory.Interactions) - count
	if start < 0 {
		start = 0
	}

	for i := start; i < len(memory.Interactions); i++ {
		if memory.Interactions[i].Emotion != "" {
			emotions = append(emotions, memory.Interactions[i].Emotion)
		}
	}

	return emotions
}

// GetConversationPattern 获取对话模式
func (mm *MemoryManager) GetConversationPattern(userID int64) string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	memory := mm.memories[userID]
	if memory == nil {
		return "新用户"
	}

	memory.mu.RLock()
	defer memory.mu.RUnlock()

	// 分析最近的交互模式
	recentCount := 10
	if len(memory.Interactions) < recentCount {
		recentCount = len(memory.Interactions)
	}

	if recentCount == 0 {
		return "新用户"
	}

	// 统计情感分布
	emotionCount := make(map[string]int)
	for i := len(memory.Interactions) - recentCount; i < len(memory.Interactions); i++ {
		emotion := memory.Interactions[i].Emotion
		if emotion != "" {
			emotionCount[emotion]++
		}
	}

	// 判断主要情感
	maxCount := 0
	mainEmotion := ""
	for emotion, count := range emotionCount {
		if count > maxCount {
			maxCount = count
			mainEmotion = emotion
		}
	}

	// 根据主要情感返回模式
	switch mainEmotion {
	case "难过":
		return "需要关怀"
	case "开心":
		return "积极活跃"
	case "生气":
		return "情绪波动"
	default:
		return "平稳交流"
	}
}

// SuggestResponse 基于记忆建议回复
func (mm *MemoryManager) SuggestResponse(userID int64, currentMsg, emotion string) string {
	pattern := mm.GetConversationPattern(userID)
	recentEmotions := mm.GetRecentEmotions(userID, 5)

	// 基于历史情感和当前情感建议回复
	switch pattern {
	case "需要关怀":
		if emotion == "难过" {
			return "我一直都在这里陪着你"
		}
		return "看起来你心情好一些了"

	case "积极活跃":
		if emotion == "开心" {
			return "哈哈，你的好心情也感染到我了"
		}
		return "怎么了，遇到什么事了吗"

	case "情绪波动":
		return "我们慢慢聊，不着急"

	default:
		// 检查是否有情感变化趋势
		if len(recentEmotions) >= 2 {
			if recentEmotions[len(recentEmotions)-1] != recentEmotions[len(recentEmotions)-2] {
				return "感觉你的心情有些变化"
			}
		}
		return ""
	}
}

func (mm *MemoryManager) generateContext(memory *EmotionalMemory) string {
	if len(memory.Interactions) == 0 {
		return "初次对话"
	}

	recent := memory.Interactions[len(memory.Interactions)-1]
	return "上次聊到: " + recent.UserMsg[:min(20, len(recent.UserMsg))]
}

func (mm *MemoryManager) updatePreferences(memory *EmotionalMemory, emotion, intention string) {
	// 更新情感偏好统计
	memory.Preferences.EmotionCount[emotion]++
	// 更新意图偏好统计
	memory.Preferences.IntentionCount[intention]++
}

func (mm *MemoryManager) saveToFile() {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// 确保目录存在
	err := os.MkdirAll("./public/memory", 0o755)
	if err != nil {
		utils.Error("创建目录失败: %v", err)
		return
	}

	data, err := json.MarshalIndent(mm.memories, "", "  ")
	if err != nil {
		utils.Error("序列化失败: %v", err)
		return
	}

	err = os.WriteFile(mm.filePath, data, 0o644)
	if err != nil {
		utils.Error("写入文件失败: %v", err)
	}
}

func (mm *MemoryManager) loadFromFile() {
	data, err := os.ReadFile(mm.filePath)
	if err != nil {
		return
	}

	_ = json.Unmarshal(data, &mm.memories)
}
