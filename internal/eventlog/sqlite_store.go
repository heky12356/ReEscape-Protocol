package eventlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SQLiteStore persists events as queryable rows while retaining the complete
// event payload for forward-compatible fields.
type SQLiteStore struct {
	database *sql.DB
}

func NewSQLiteEventStore(database *sql.DB) *SQLiteStore {
	return &SQLiteStore{database: database}
}

func (s *SQLiteStore) Append(ctx context.Context, events ...Event) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("SQLite event store is not configured")
	}
	prepared := normalizeEvents(events)
	if len(prepared) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite event append: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, event := range prepared {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", event.ID, err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO events(
            id, type, created_at, user_id, session_id, turn_id, intent_id, delivery_id, payload_json
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			event.ID, event.Type, event.CreatedAt.UTC().Format(time.RFC3339Nano), event.UserID,
			event.SessionID, event.TurnID, event.IntentID, event.DeliveryID, string(payload))
		if err != nil {
			return fmt.Errorf("append SQLite event %s: %w", event.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite events: %w", err)
	}
	return nil
}

func (s *SQLiteStore) List(ctx context.Context, limit int) ([]Event, error) {
	if s == nil || s.database == nil {
		return nil, fmt.Errorf("SQLite event store is not configured")
	}
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.database.QueryContext(ctx,
		"SELECT cursor, payload_json FROM events ORDER BY cursor DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("list SQLite events: %w", err)
	}
	defer rows.Close()
	events, err := scanSQLiteEvents(rows)
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	return events, nil
}

func (s *SQLiteStore) ListAfter(ctx context.Context, cursor int64, limit int) ([]Event, error) {
	if s == nil || s.database == nil {
		return nil, fmt.Errorf("SQLite event store is not configured")
	}
	if cursor < 0 {
		cursor = 0
	}
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.database.QueryContext(ctx,
		"SELECT cursor, payload_json FROM events WHERE cursor > ? ORDER BY cursor ASC LIMIT ?", cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list SQLite events after cursor %d: %w", cursor, err)
	}
	defer rows.Close()
	return scanSQLiteEvents(rows)
}

func (s *SQLiteStore) LatestCursor(ctx context.Context) (int64, error) {
	if s == nil || s.database == nil {
		return 0, fmt.Errorf("SQLite event store is not configured")
	}
	var cursor int64
	if err := s.database.QueryRowContext(ctx, "SELECT COALESCE(MAX(cursor), 0) FROM events").Scan(&cursor); err != nil {
		return 0, fmt.Errorf("read latest SQLite event cursor: %w", err)
	}
	return cursor, nil
}

func scanSQLiteEvents(rows *sql.Rows) ([]Event, error) {
	events := make([]Event, 0)
	for rows.Next() {
		var cursor int64
		var payload string
		if err := rows.Scan(&cursor, &payload); err != nil {
			return nil, fmt.Errorf("scan SQLite event: %w", err)
		}
		var event Event
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return nil, fmt.Errorf("decode SQLite event at cursor %d: %w", cursor, err)
		}
		event.Cursor = cursor
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite events: %w", err)
	}
	return events, nil
}
