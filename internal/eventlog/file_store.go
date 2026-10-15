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
	mu         sync.RWMutex
	events     []Event
	nextCursor int64
	store      storage.SnapshotStore
	dirty      storage.DirtyMarker
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
	for i := range fs.events {
		if fs.events[i].Cursor <= fs.nextCursor {
			fs.nextCursor++
			fs.events[i].Cursor = fs.nextCursor
		} else {
			fs.nextCursor = fs.events[i].Cursor
		}
	}
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

	prepared := normalizeEvents(events)
	if len(prepared) == 0 {
		return nil
	}
	s.mu.Lock()
	for _, event := range prepared {
		s.nextCursor++
		event.Cursor = s.nextCursor
		s.events = append(s.events, event)
	}
	s.trimLocked()
	s.mu.Unlock()

	if s.dirty != nil {
		s.dirty.MarkDirty(FlushTaskName)
	}
	return nil
}

func normalizeEvents(events []Event) []Event {
	now := time.Now()
	prepared := make([]Event, 0, len(events))
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
		if event.ToolName == "" {
			event.ToolName = event.Tool
		}
		if event.Tool == "" {
			event.Tool = event.ToolName
		}
		prepared = append(prepared, event)
	}
	return prepared
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

func (s *FileStore) ListAfter(ctx context.Context, cursor int64, limit int) ([]Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if cursor < 0 {
		cursor = 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	start := len(s.events)
	for i, event := range s.events {
		if event.Cursor > cursor {
			start = i
			break
		}
	}
	if start == len(s.events) {
		return []Event{}, nil
	}
	end := len(s.events)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	result := make([]Event, end-start)
	copy(result, s.events[start:end])
	return result, nil
}

func (s *FileStore) LatestCursor(ctx context.Context) (int64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextCursor, nil
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

func (s *FileStore) Replay(ctx context.Context, options ReplayOptions) (ReplayPage, error) {
	if err := ctx.Err(); err != nil {
		return ReplayPage{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	page := ReplayPage{Events: []Event{}, Latest: s.nextCursor, Cursor: s.nextCursor}
	if len(s.events) > 0 {
		page.Earliest = s.events[0].Cursor
	}
	limit := replayLimit(options.Limit)
	start, end := 0, len(s.events)
	if options.After != nil {
		after := *options.After
		if after < 0 {
			return page, fmt.Errorf("negative event cursor")
		}
		if after > page.Latest {
			page.Reason = "cursor_reset"
			return page, nil
		}
		if page.Earliest > 0 && after < page.Earliest-1 {
			page.Reason = "retention_gap"
			return page, nil
		}
		page.Cursor = after
		for start < end && s.events[start].Cursor <= after {
			start++
		}
		if end-start > limit {
			end = start + limit
			page.HasMore = true
		}
		if end > start {
			page.Cursor = s.events[end-1].Cursor
		}
	} else if end > limit {
		start = end - limit
	}
	page.Events = append(page.Events, s.events[start:end]...)
	filterReplay(&page, options.Filters)
	return page, nil
}

func (s *FileStore) trimLocked() {
	if len(s.events) <= maxEvents {
		return
	}
	s.events = append([]Event(nil), s.events[len(s.events)-maxEvents:]...)
}
