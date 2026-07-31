package eventlog

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/storage"
	"project-yume/internal/utils"
)

const (
	SnapshotName  = "events/react_event_log.json"
	FlushTaskName = "eventlog"
	maxEvents     = 1000
)

type FileStore struct {
	mu     sync.RWMutex
	events []Event
	store  storage.SnapshotStore
	dirty  storage.DirtyMarker
}

type eventStorage struct {
	Events []Event `json:"events"`
}

func NewFileStore(store storage.SnapshotStore, dirty storage.DirtyMarker) (*FileStore, error) {
	fs := &FileStore{
		store: store,
		dirty: dirty,
	}
	if store == nil {
		return fs, nil
	}
	data, err := store.Load(SnapshotName)
	if err != nil {
		return nil, fmt.Errorf("load event log failed: %w", err)
	}
	if len(data) == 0 {
		return fs, nil
	}
	var snapshot eventStorage
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("parse event log failed: %w", err)
	}
	fs.events = append([]Event(nil), snapshot.Events...)
	fs.trimLocked()
	return fs, nil
}

func (s *FileStore) Append(ctx context.Context, events ...Event) error {
	if len(events) == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	now := time.Now()
	s.mu.Lock()
	for _, event := range events {
		if event.Type == "" {
			continue
		}
		if event.ID == "" {
			event.ID = utils.NewRequestID("evt")
		}
		if event.CreatedAt.IsZero() {
			event.CreatedAt = now
		}
		s.events = append(s.events, event)
	}
	s.trimLocked()
	s.mu.Unlock()

	if s.dirty != nil {
		s.dirty.MarkDirty(FlushTaskName)
	}
	return nil
}

func (s *FileStore) List(ctx context.Context, limit int) ([]Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	start := len(s.events) - limit
	result := make([]Event, limit)
	copy(result, s.events[start:])
	return result, nil
}

func (s *FileStore) Flush() error {
	s.mu.RLock()
	events := make([]Event, len(s.events))
	copy(events, s.events)
	store := s.store
	s.mu.RUnlock()

	if store == nil {
		return nil
	}

	data, err := json.MarshalIndent(eventStorage{Events: events}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal event log failed: %w", err)
	}
	return store.Save(SnapshotName, data)
}

func (s *FileStore) trimLocked() {
	if len(s.events) <= maxEvents {
		return
	}
	s.events = append([]Event(nil), s.events[len(s.events)-maxEvents:]...)
}
