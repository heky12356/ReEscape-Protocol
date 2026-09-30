package intent

import "time"

type Store interface {
	List(userID int64, sessionID string, statuses ...Status) []Intent
	Get(id string) (Intent, bool)
	Upsert(item Intent) (Intent, error)
	ClaimDue(userID int64, sessionID string, now time.Time) (Intent, bool)
	Transition(id string, to Status, now time.Time) (Intent, error)
}
