package openloop

import (
	"fmt"
	"time"
)

func FromState(userID int64, sessionID string, kind, description string, turnIndex int) OpenLoop {
	return Normalize(OpenLoop{
		ID:          fmt.Sprintf("%s:%s:%d", sessionID, kind, turnIndex),
		UserID:      userID,
		SessionID:   sessionID,
		Kind:        Kind(kind),
		Description: description,
		CreatedAt:   time.Now(),
	})
}
