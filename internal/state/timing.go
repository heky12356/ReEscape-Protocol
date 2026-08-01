package state

import "time"

// SessionTimingSnapshot captures session timing before the current turn mutates it.
type SessionTimingSnapshot struct {
	LastUserMessageAt      time.Time
	LastAssistantMessageAt time.Time
	LastInteractionAt      time.Time
	LastReply              time.Time
}

// GetTimingSnapshot returns the previous known timing values for a session.
func (sm *StateManager) GetTimingSnapshot(sessionID string) SessionTimingSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil || len(session.Conversation) == 0 {
		return SessionTimingSnapshot{}
	}

	return SessionTimingSnapshot{
		LastUserMessageAt:      session.LastUserMessageAt,
		LastAssistantMessageAt: session.LastAssistantMessageAt,
		LastInteractionAt:      session.LastInteractionAt,
		LastReply:              session.LastReply,
	}
}
