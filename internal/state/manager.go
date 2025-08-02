package state

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// BotState 机器人状态枚举
type BotState int

const (
	// 空闲状态
	StateIdle BotState = iota
	StateNeedComfort
	// 需求情感状态
	StateNeedEncourage
	// 长对话状态
	StateLongChat
	// 随机状态
	StatePerfunctory
	// 忙碌状态
	StateBusy
)

// ConversationData 对话数据
type ConversationData struct {
	UserID       int64                           `json:"user_id"`
	Conversation []openai.ChatCompletionMessage `json:"conversation"`
	LastUpdated  time.Time                      `json:"last_updated"`
}

// ConversationStorage 对话存储
type ConversationStorage struct {
	Conversations map[int64]*ConversationData `json:"conversations"`
}

// StateManager 状态管理器
type StateManager struct {
	mu               sync.RWMutex
	currentState     BotState
	flags            map[string]bool
	counters         map[string]int
	lastReply        time.Time
	conversations    map[int64]*ConversationData // 按用户ID存储对话历史
	conversationFile string                      // 对话历史文件路径
}

var manager *StateManager

// 初始化
func init() {
	manager = &StateManager{
		currentState:     StateIdle,
		flags:            make(map[string]bool),
		counters:         make(map[string]int),
		lastReply:        time.Now(),
		conversations:    make(map[int64]*ConversationData),
		conversationFile: "./public/memory/conversation_history.json",
	}
	manager.loadConversationsFromFile()
}

// GetManager 获取状态管理器实例
func GetManager() *StateManager {
	return manager
}

// SetState 设置当前状态
func (sm *StateManager) SetState(state BotState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentState = state
}

// GetState 获取当前状态
func (sm *StateManager) GetState() BotState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

func (sm *StateManager) IncrementCounter(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.counters[key]++
}

func (sm *StateManager) GetCounter(key string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.counters[key]
}

func (sm *StateManager) UpdateLastReply() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastReply = time.Now()
}

// GetTimeSinceLastReply 获取上次回复时间距离现在的时长
func (sm *StateManager) GetTimeSinceLastReply() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.lastReply)
}

// 对话管理方法
func (sm *StateManager) AddToConversation(userID int64, msg openai.ChatCompletionMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.conversations[userID] == nil {
		sm.conversations[userID] = &ConversationData{
			UserID:       userID,
			Conversation: make([]openai.ChatCompletionMessage, 0),
			LastUpdated:  time.Now(),
		}
	}
	
	sm.conversations[userID].Conversation = append(sm.conversations[userID].Conversation, msg)
	sm.conversations[userID].LastUpdated = time.Now()
	
	// 异步保存到文件
	go sm.saveConversationsToFile()
}

// GetConversation 获取对话历史
func (sm *StateManager) GetConversation(userID int64) []openai.ChatCompletionMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if sm.conversations[userID] == nil {
		return []openai.ChatCompletionMessage{}
	}
	
	// 返回副本以避免并发修改
	result := make([]openai.ChatCompletionMessage, len(sm.conversations[userID].Conversation))
	copy(result, sm.conversations[userID].Conversation)
	return result
}

// SetConversation 设置对话历史
func (sm *StateManager) SetConversation(userID int64, conversation []openai.ChatCompletionMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.conversations[userID] == nil {
		sm.conversations[userID] = &ConversationData{
			UserID:      userID,
			LastUpdated: time.Now(),
		}
	}
	
	sm.conversations[userID].Conversation = conversation
	sm.conversations[userID].LastUpdated = time.Now()
	
	// 异步保存到文件
	go sm.saveConversationsToFile()
}

// ClearConversation 清空对话历史
func (sm *StateManager) ClearConversation(userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.conversations[userID] != nil {
		sm.conversations[userID].Conversation = nil
		sm.conversations[userID].LastUpdated = time.Now()
		
		// 异步保存到文件
		go sm.saveConversationsToFile()
	}
}

// ClearAllConversations 清空所有对话历史
func (sm *StateManager) ClearAllConversations() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.conversations = make(map[int64]*ConversationData)
	
	// 异步保存到文件
	go sm.saveConversationsToFile()
}

// 重置计数器
func (sm *StateManager) ResetCounter(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.counters[key] = 0
}

// 设置计数器值
func (sm *StateManager) SetCounter(key string, value int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.counters[key] = value
}

// 获取所有标志状态（用于调试）
func (sm *StateManager) GetAllFlags() map[string]bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]bool)
	for k, v := range sm.flags {
		result[k] = v
	}
	return result
}

// 重置所有状态
func (sm *StateManager) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentState = StateIdle
	sm.flags = make(map[string]bool)
	sm.counters = make(map[string]int)
	sm.conversations = make(map[int64]*ConversationData)
	sm.lastReply = time.Now()
	
	// 异步保存到文件
	go sm.saveConversationsToFile()
}

// saveConversationsToFile 保存对话历史到文件
func (sm *StateManager) saveConversationsToFile() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 确保目录存在
	err := os.MkdirAll("./public/memory", 0o755)
	if err != nil {
		log.Printf("创建目录失败: %v", err)
		return
	}

	storage := ConversationStorage{
		Conversations: sm.conversations,
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		log.Printf("序列化对话历史失败: %v", err)
		return
	}

	err = os.WriteFile(sm.conversationFile, data, 0o644)
	if err != nil {
		log.Printf("写入对话历史文件失败: %v", err)
	}
}

// loadConversationsFromFile 从文件加载对话历史
func (sm *StateManager) loadConversationsFromFile() {
	data, err := os.ReadFile(sm.conversationFile)
	if err != nil {
		// 文件不存在是正常的，不需要报错
		return
	}

	var storage ConversationStorage
	err = json.Unmarshal(data, &storage)
	if err != nil {
		log.Printf("解析对话历史文件失败: %v", err)
		return
	}

	sm.conversations = storage.Conversations
	if sm.conversations == nil {
		sm.conversations = make(map[int64]*ConversationData)
	}
}
