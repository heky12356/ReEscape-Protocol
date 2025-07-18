package state

import (
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// BotState 机器人状态枚举
type BotState int

const (
	StateIdle BotState = iota
	StateNeedComfort
	StateNeedEncourage
	StateLongChat
	StatePerfunctory
)

// 状态标志常量
const (
	FlagLongChain     = "long_chain"
	FlagNeedComfort   = "need_comfort"
	FlagNeedEncourage = "need_encourage"
	FlagComfort       = "comfort"
	FlagEncourage     = "encourage"
	FlagPerfunctory   = "perfunctory"
	FlagBusy          = "busy"
	FlagSleep         = "sleep"
)

// StateManager 状态管理器
type StateManager struct {
	mu           sync.RWMutex
	currentState BotState
	flags        map[string]bool
	counters     map[string]int
	lastReply    time.Time
	conversation []openai.ChatCompletionMessage // AI对话历史
}

var manager *StateManager

func init() {
	manager = &StateManager{
		currentState: StateIdle,
		flags:        make(map[string]bool),
		counters:     make(map[string]int),
		lastReply:    time.Now(),
	}
}

func GetManager() *StateManager {
	return manager
}

func (sm *StateManager) SetState(state BotState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentState = state
}

func (sm *StateManager) GetState() BotState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

func (sm *StateManager) SetFlag(key string, value bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.flags[key] = value
}

func (sm *StateManager) GetFlag(key string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.flags[key]
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

func (sm *StateManager) GetTimeSinceLastReply() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.lastReply)
}

// 对话管理方法
func (sm *StateManager) AddToConversation(msg openai.ChatCompletionMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.conversation = append(sm.conversation, msg)
}

func (sm *StateManager) GetConversation() []openai.ChatCompletionMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	// 返回副本以避免并发修改
	result := make([]openai.ChatCompletionMessage, len(sm.conversation))
	copy(result, sm.conversation)
	return result
}

func (sm *StateManager) SetConversation(conversation []openai.ChatCompletionMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.conversation = conversation
}

func (sm *StateManager) ClearConversation() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.conversation = nil
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
	sm.conversation = nil
	sm.lastReply = time.Now()
}
