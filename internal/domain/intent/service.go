package intent

import (
	"fmt"
	"time"
)

func NewProactiveContact(userID int64, sessionID, summary string, dueAt time.Time) Intent {
	return Normalize(Intent{
		ID:     fmt.Sprintf("intent:%d:%d", userID, time.Now().UnixNano()),
		UserID: userID, SessionID: sessionID, Kind: KindProactiveContact,
		Summary: summary, Action: string(KindProactiveContact), DueAt: dueAt, Source: "scheduler",
		CreatedAt: time.Now(),
	})
}
