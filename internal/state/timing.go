package state

import "time"

// SessionTimingSnapshot captures session timing before the current turn mutates it.
type SessionTimingSnapshot struct {
	LastUserMessageAt      time.Time
	LastAssistantMessageAt time.Time
	LastInteractionAt      time.Time
	LastReply              time.Time
	PendingUserTurnAt      time.Time
	UserTurnInFlight       bool
	ProactiveTurnInFlight  bool
	ProactiveClaimedAt     time.Time
}

// GetTimingSnapshot returns the previous known timing values for a session.
func (sm *StateManager) GetTimingSnapshot(sessionID string) SessionTimingSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return SessionTimingSnapshot{}
	}

	snapshot := SessionTimingSnapshot{
		PendingUserTurnAt:     session.PendingUserTurnAt,
		UserTurnInFlight:      session.UserTurnInFlight,
		ProactiveTurnInFlight: session.ProactiveTurnInFlight,
		ProactiveClaimedAt:    session.ProactiveClaimedAt,
	}
	if len(session.Conversation) == 0 {
		return snapshot
	}

	snapshot.LastUserMessageAt = session.LastUserMessageAt
	snapshot.LastAssistantMessageAt = session.LastAssistantMessageAt
	snapshot.LastInteractionAt = session.LastInteractionAt
	snapshot.LastReply = session.LastReply
	return snapshot
}
