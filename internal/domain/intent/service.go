package intent

import "time"

func NewProactiveContact(userID int64, sessionID, summary string, dueAt time.Time) Intent {
	return Normalize(Intent{
		UserID: userID, SessionID: sessionID, Kind: KindProactiveContact,
		Summary: summary, DueAt: dueAt, Source: "scheduler",
	})
}
