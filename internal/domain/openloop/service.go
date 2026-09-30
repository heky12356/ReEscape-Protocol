package openloop

import (
	"fmt"
	"time"
)

func FromState(userID int64, sessionID string, kind, description string, turnIndex int) OpenLoop {
	return Normalize(OpenLoop{
		ID:           fmt.Sprintf("%s:%s:%d", sessionID, kind, turnIndex),
		UserID:       userID,
		SessionID:    sessionID,
		Kind:         Kind(kind),
		Description:  description,
		Source:       "heuristic",
		SourceTurnID: fmt.Sprintf("turn:%d", turnIndex),
		TurnIndex:    turnIndex,
		CreatedAt:    time.Now(),
	})
}

func NewExplicit(userID int64, sessionID, kind, description, sourceTurnID string, now time.Time) OpenLoop {
	if now.IsZero() {
		now = time.Now()
	}
	return Normalize(OpenLoop{
		ID:           fmt.Sprintf("openloop:%d:%d", userID, now.UnixNano()),
		UserID:       userID,
		SessionID:    sessionID,
		Kind:         Kind(kind),
		Description:  description,
		Source:       "explicit",
		SourceTurnID: sourceTurnID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}
