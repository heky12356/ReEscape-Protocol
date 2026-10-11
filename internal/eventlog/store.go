package eventlog

import (
	"context"
	"sync"
)

type Store interface {
	Append(ctx context.Context, events ...Event) error
	List(ctx context.Context, limit int) ([]Event, error)
}

// CursorStore supports incremental event replay for consumers that reconnect
// after receiving an earlier cursor.
type CursorStore interface {
	Store
	ListAfter(ctx context.Context, cursor int64, limit int) ([]Event, error)
	LatestCursor(ctx context.Context) (int64, error)
}

var (
	defaultStoreMu sync.RWMutex
	defaultStore   Store
)

// SetDefault keeps the admin diagnostic path wired to the app-level event log.
// Turn processing should still receive Store explicitly so tests and future multi-instance
// runs are not coupled to this singleton.
func SetDefault(store Store) {
	defaultStoreMu.Lock()
	defaultStore = store
	defaultStoreMu.Unlock()
}

func Default() Store {
	defaultStoreMu.RLock()
	defer defaultStoreMu.RUnlock()
	return defaultStore
}
