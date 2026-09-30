package openloop

import "time"

type Store interface {
	List(userID int64, sessionID string, includeClosed bool) []OpenLoop
	Get(id string) (OpenLoop, bool)
	Upsert(loop OpenLoop) (OpenLoop, error)
	Close(id, actor string, now time.Time) (OpenLoop, error)
	Defer(id string, dueAt time.Time, actor string, now time.Time) (OpenLoop, error)
}
