package state

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"

	"project-yume/internal/storage"
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

// Session 保存单个用户/会话的运行态。
type Session struct {
	ID           string                         `json:"id"`
	UserID       int64                          `json:"user_id"`
	GroupID      int64                          `json:"group_id"`
	ChatType     int                            `json:"chat_type"`
	CurrentState BotState                       `json:"current_state"`
	Flags        map[string]bool                `json:"flags"`
	Counters     map[string]int                 `json:"counters"`
	LastReply    time.Time                      `json:"last_reply"`
	Conversation []openai.ChatCompletionMessage `json:"conversation"`
	Summary      string                         `json:"summary"`
	ActiveTopics []string                       `json:"active_topics"`
	LastUpdated  time.Time                      `json:"last_updated"`
}

// SessionStorage 会话存储。
type SessionStorage struct {
	Sessions map[string]*Session `json:"sessions"`
}

type legacyConversationData struct {
	UserID       int64                          `json:"user_id"`
	Conversation []openai.ChatCompletionMessage `json:"conversation"`
	LastUpdated  time.Time                      `json:"last_updated"`
}

type legacyConversationStorage struct {
	Conversations map[int64]*legacyConversationData `json:"conversations"`
}

// StateManager 状态管理器
type StateManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	store    storage.SnapshotStore
	dirty    storage.DirtyMarker
}

var manager *StateManager

const SnapshotName = "memory/conversation_history.json"
const FlushTaskName = "sessions"

// 初始化
func init() {
	manager = &StateManager{
		sessions: make(map[string]*Session),
	}
}

// GetManager 获取状态管理器实例
func GetManager() *StateManager {
	return manager
}

// BuildSessionID 为私聊/群聊用户构造稳定的 session id。
func BuildSessionID(userID, groupID int64, chatType int) string {
	if chatType == 0 {
		return fmt.Sprintf("group:%d:user:%d", groupID, userID)
	}
	return PrivateSessionID(userID)
}

// PrivateSessionID 返回私聊会话 id。
func PrivateSessionID(userID int64) string {
	return fmt.Sprintf("private:%d", userID)
}

// EnsureSession 确保会话存在，并补齐元信息。
func (sm *StateManager) EnsureSession(sessionID string, userID, groupID int64, chatType int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.ensureSessionLocked(sessionID, userID, groupID, chatType)
}

// SetState 设置当前状态
func (sm *StateManager) SetState(sessionID string, state BotState) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.CurrentState = state
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

// GetState 获取当前状态
func (sm *StateManager) GetState(sessionID string) BotState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return StateIdle
	}
	return session.CurrentState
}

func (sm *StateManager) IncrementCounter(sessionID, key string) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.Counters[key]++
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

func (sm *StateManager) GetCounter(sessionID, key string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return 0
	}
	return session.Counters[key]
}

func (sm *StateManager) UpdateLastReply(sessionID string) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	now := time.Now()
	session.LastReply = now
	session.LastUpdated = now
	sm.mu.Unlock()

	sm.markDirty()
}

// GetTimeSinceLastReply 获取上次回复时间距离现在的时长
func (sm *StateManager) GetTimeSinceLastReply(sessionID string) time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil || session.LastReply.IsZero() {
		return 0
	}
	return time.Since(session.LastReply)
}

// AddToConversation 追加对话历史。
func (sm *StateManager) AddToConversation(sessionID string, msg openai.ChatCompletionMessage) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.Conversation = append(session.Conversation, msg)
	refreshSessionDerivedMemory(session)
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

// GetConversation 获取对话历史
func (sm *StateManager) GetConversation(sessionID string) []openai.ChatCompletionMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return []openai.ChatCompletionMessage{}
	}

	result := make([]openai.ChatCompletionMessage, len(session.Conversation))
	copy(result, session.Conversation)
	return result
}

// SetConversation 设置对话历史
func (sm *StateManager) SetConversation(sessionID string, conversation []openai.ChatCompletionMessage) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.Conversation = append([]openai.ChatCompletionMessage(nil), conversation...)
	refreshSessionDerivedMemory(session)
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

// ClearConversation 清空对话历史
func (sm *StateManager) ClearConversation(sessionID string) {
	sm.mu.Lock()
	session := sm.sessions[sessionID]
	if session != nil {
		session.Conversation = []openai.ChatCompletionMessage{}
		session.Summary = ""
		session.ActiveTopics = []string{}
		session.LastUpdated = time.Now()
	}
	sm.mu.Unlock()

	sm.markDirty()
}

// ClearAllSessions 清空所有会话
func (sm *StateManager) ClearAllSessions() {
	sm.mu.Lock()
	sm.sessions = make(map[string]*Session)
	sm.mu.Unlock()

	sm.markDirty()
}

// ResetCounter 重置会话内的某个计数器
func (sm *StateManager) ResetCounter(sessionID, key string) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.Counters[key] = 0
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

// SetCounter 设置会话内的计数器值
func (sm *StateManager) SetCounter(sessionID, key string, value int) {
	sm.mu.Lock()
	session := sm.ensureSessionLocked(sessionID, 0, 0, 0)
	session.Counters[key] = value
	session.LastUpdated = time.Now()
	sm.mu.Unlock()

	sm.markDirty()
}

// GetAllFlags 获取会话内所有标志状态
func (sm *StateManager) GetAllFlags(sessionID string) map[string]bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]bool)
	session := sm.sessions[sessionID]
	if session == nil {
		return result
	}
	for key, value := range session.Flags {
		result[key] = value
	}
	return result
}

// ResetSession 重置指定会话的所有状态。
func (sm *StateManager) ResetSession(sessionID string) {
	sm.mu.Lock()
	session := sm.sessions[sessionID]
	if session != nil {
		session.CurrentState = StateIdle
		session.Flags = make(map[string]bool)
		session.Counters = make(map[string]int)
		session.Conversation = []openai.ChatCompletionMessage{}
		session.Summary = ""
		session.ActiveTopics = []string{}
		session.LastReply = time.Now()
		session.LastUpdated = time.Now()
	}
	sm.mu.Unlock()

	sm.markDirty()
}

func (sm *StateManager) ensureSessionLocked(sessionID string, userID, groupID int64, chatType int) *Session {
	session := sm.sessions[sessionID]
	if session == nil {
		now := time.Now()
		session = &Session{
			ID:           sessionID,
			UserID:       userID,
			GroupID:      groupID,
			ChatType:     chatType,
			CurrentState: StateIdle,
			Flags:        make(map[string]bool),
			Counters:     make(map[string]int),
			LastReply:    now,
			Conversation: []openai.ChatCompletionMessage{},
			ActiveTopics: []string{},
			LastUpdated:  now,
		}
		sm.sessions[sessionID] = session
		return session
	}

	if userID != 0 {
		session.UserID = userID
	}
	if groupID != 0 {
		session.GroupID = groupID
	}
	if chatType == 0 || chatType == 1 {
		session.ChatType = chatType
	}
	if session.Flags == nil {
		session.Flags = make(map[string]bool)
	}
	if session.Counters == nil {
		session.Counters = make(map[string]int)
	}
	if session.Conversation == nil {
		session.Conversation = []openai.ChatCompletionMessage{}
	}
	if session.ActiveTopics == nil {
		session.ActiveTopics = []string{}
	}
	if session.LastReply.IsZero() {
		session.LastReply = time.Now()
	}
	if session.LastUpdated.IsZero() {
		session.LastUpdated = session.LastReply
	}
	if session.ID == "" {
		session.ID = sessionID
	}

	return session
}

func (sm *StateManager) ConfigurePersistence(store storage.SnapshotStore, dirty storage.DirtyMarker) error {
	sm.mu.Lock()
	sm.store = store
	sm.dirty = dirty
	sm.mu.Unlock()

	if store == nil {
		return nil
	}

	data, err := store.Load(SnapshotName)
	if err != nil {
		return fmt.Errorf("load sessions failed: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	return sm.loadSessionsFromBytes(data)
}

func (sm *StateManager) Flush() error {
	sm.mu.RLock()
	store := sm.store
	snapshot := sm.snapshotLocked()
	sm.mu.RUnlock()

	if store == nil {
		return nil
	}

	data, err := json.MarshalIndent(SessionStorage{Sessions: snapshot}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions failed: %w", err)
	}

	if err := store.Save(SnapshotName, data); err != nil {
		return fmt.Errorf("save sessions failed: %w", err)
	}

	return nil
}

func (sm *StateManager) loadSessionsFromBytes(data []byte) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("解析会话历史文件失败: %w", err)
	}

	if rawSessions, ok := envelope["sessions"]; ok {
		var storage SessionStorage
		if err := json.Unmarshal(rawSessions, &storage.Sessions); err != nil {
			return fmt.Errorf("解析会话存储失败: %w", err)
		}
		sm.mu.Lock()
		sm.sessions = storage.Sessions
		sm.normalizeSessions()
		sm.mu.Unlock()
		return nil
	}

	if rawConversations, ok := envelope["conversations"]; ok {
		var legacy legacyConversationStorage
		if err := json.Unmarshal(rawConversations, &legacy.Conversations); err != nil {
			return fmt.Errorf("解析旧版对话历史失败: %w", err)
		}
		sm.mu.Lock()
		sm.sessions = migrateLegacyConversations(legacy.Conversations)
		sm.normalizeSessions()
		sm.mu.Unlock()
		return nil
	}

	return fmt.Errorf("unknown session storage format")
}

func (sm *StateManager) normalizeSessions() {
	if sm.sessions == nil {
		sm.sessions = make(map[string]*Session)
		return
	}

	for sessionID, session := range sm.sessions {
		if session == nil {
			delete(sm.sessions, sessionID)
			continue
		}
		sm.normalizeSession(sessionID, session)
	}
}

func (sm *StateManager) normalizeSession(sessionID string, session *Session) {
	if session.ID == "" {
		session.ID = sessionID
	}
	if session.Flags == nil {
		session.Flags = make(map[string]bool)
	}
	if session.Counters == nil {
		session.Counters = make(map[string]int)
	}
	if session.Conversation == nil {
		session.Conversation = []openai.ChatCompletionMessage{}
	}
	if session.ActiveTopics == nil {
		session.ActiveTopics = []string{}
	}
	if session.LastReply.IsZero() {
		if session.LastUpdated.IsZero() {
			session.LastReply = time.Now()
		} else {
			session.LastReply = session.LastUpdated
		}
	}
	if session.LastUpdated.IsZero() {
		session.LastUpdated = session.LastReply
	}
	if session.Summary == "" && len(session.Conversation) > 0 {
		refreshSessionDerivedMemory(session)
	}
}

func migrateLegacyConversations(conversations map[int64]*legacyConversationData) map[string]*Session {
	sessions := make(map[string]*Session, len(conversations))

	for userID, conversation := range conversations {
		if conversation == nil {
			continue
		}

		sessionID := PrivateSessionID(userID)
		lastUpdated := conversation.LastUpdated
		if lastUpdated.IsZero() {
			lastUpdated = time.Now()
		}

		sessions[sessionID] = &Session{
			ID:           sessionID,
			UserID:       userID,
			ChatType:     1,
			CurrentState: StateIdle,
			Flags:        make(map[string]bool),
			Counters:     make(map[string]int),
			LastReply:    lastUpdated,
			Conversation: append([]openai.ChatCompletionMessage(nil), conversation.Conversation...),
			ActiveTopics: []string{},
			LastUpdated:  lastUpdated,
		}
		refreshSessionDerivedMemory(sessions[sessionID])
	}

	return sessions
}

func (sm *StateManager) markDirty() {
	sm.mu.RLock()
	dirty := sm.dirty
	sm.mu.RUnlock()

	if dirty != nil {
		dirty.MarkDirty(FlushTaskName)
	}
}

func (sm *StateManager) snapshotLocked() map[string]*Session {
	result := make(map[string]*Session, len(sm.sessions))

	for sessionID, session := range sm.sessions {
		if session == nil {
			continue
		}
		result[sessionID] = &Session{
			ID:           session.ID,
			UserID:       session.UserID,
			GroupID:      session.GroupID,
			ChatType:     session.ChatType,
			CurrentState: session.CurrentState,
			Flags:        cloneBoolMap(session.Flags),
			Counters:     cloneIntMap(session.Counters),
			LastReply:    session.LastReply,
			Conversation: append([]openai.ChatCompletionMessage(nil), session.Conversation...),
			Summary:      session.Summary,
			ActiveTopics: append([]string(nil), session.ActiveTopics...),
			LastUpdated:  session.LastUpdated,
		}
	}

	return result
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneIntMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (sm *StateManager) GetConversationSummary(sessionID string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return ""
	}
	return session.Summary
}

func (sm *StateManager) GetActiveTopics(sessionID string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return []string{}
	}
	return append([]string(nil), session.ActiveTopics...)
}

func refreshSessionDerivedMemory(session *Session) {
	session.Summary = summarizeConversation(session.Conversation)
	session.ActiveTopics = extractActiveTopics(session.Conversation)
}

func summarizeConversation(conversation []openai.ChatCompletionMessage) string {
	if len(conversation) == 0 {
		return ""
	}

	start := 0
	if len(conversation) > 6 {
		start = len(conversation) - 6
	}

	parts := make([]string, 0, len(conversation)-start)
	for _, msg := range conversation[start:] {
		if msg.Role == "system" {
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		parts = append(parts, fmt.Sprintf("%s:%s", summarizeRole(msg.Role), truncateRunes(content, 32)))
	}

	return strings.Join(parts, " | ")
}

func extractActiveTopics(conversation []openai.ChatCompletionMessage) []string {
	topics := make([]string, 0, 3)
	seen := make(map[string]struct{})

	for i := len(conversation) - 1; i >= 0 && len(topics) < 3; i-- {
		msg := conversation[i]
		if msg.Role != "user" {
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		topic := truncateRunes(content, 16)
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		topics = append([]string{topic}, topics...)
	}

	return topics
}

func summarizeRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	default:
		return role
	}
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}

	return string(runes[:limit]) + "..."
}
