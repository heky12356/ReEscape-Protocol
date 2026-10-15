package eventlog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"project-yume/internal/storage"
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
		event.Cursor = 0
		event.CreatedAt = event.CreatedAt.UTC()
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", event.ID, err)
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO events(
            id, type, created_at, user_id, session_id, turn_id, intent_id, delivery_id, payload_json
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
			event.ID, event.Type, event.CreatedAt.UTC().Format(time.RFC3339Nano), event.UserID,
			event.SessionID, event.TurnID, event.IntentID, event.DeliveryID, string(payload))
		if err != nil {
			return fmt.Errorf("append SQLite event %s: %w", event.ID, err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			var stored []byte
			if err := tx.QueryRowContext(ctx, "SELECT payload_json FROM events WHERE id = ?", event.ID).Scan(&stored); err != nil {
				return err
			}
			existing, err := decodeSQLiteEvent(stored)
			if err != nil {
				return err
			}
			existing.Cursor, existing.CreatedAt = 0, existing.CreatedAt.UTC()
			canonical, err := json.Marshal(existing)
			if err != nil {
				return err
			}
			if !bytes.Equal(canonical, payload) {
				return fmt.Errorf("conflicting event id %s", event.ID)
			}
			continue
		}
		cursor, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if err := storage.ProjectDeliveryEvent(ctx, tx, cursor, payload); err != nil {
			return err
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
		event, err := decodeSQLiteEvent([]byte(payload))
		if err != nil {
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

func decodeSQLiteEvent(payload []byte) (Event, error) {
	var event Event
	if !json.Valid(payload) {
		return event, fmt.Errorf("invalid event JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	// Data includes OneBot message IDs which may exceed float64's exact range.
	// Preserve numbers so read/retry cannot change event content or identity.
	decoder.UseNumber()
	err := decoder.Decode(&event)
	return event, err
}

func (s *SQLiteStore) Replay(ctx context.Context, options ReplayOptions) (ReplayPage, error) {
	page := ReplayPage{Events: []Event{}}
	if s == nil || s.database == nil {
		return page, fmt.Errorf("SQLite event store is not configured")
	}
	tx, err := s.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT cursor FROM events ORDER BY cursor ASC LIMIT 1),0), COALESCE((SELECT cursor FROM events ORDER BY cursor DESC LIMIT 1),0)`).Scan(&page.Earliest, &page.Latest)
	if err != nil {
		return page, err
	}
	limit := replayLimit(options.Limit)
	var rows *sql.Rows
	if options.After == nil {
		page.Cursor = page.Latest
		rows, err = tx.QueryContext(ctx, "SELECT cursor,payload_json FROM events ORDER BY cursor DESC LIMIT ?", limit)
	} else {
		page.Cursor = *options.After
		if page.Cursor < 0 {
			return page, fmt.Errorf("negative event cursor")
		}
		if page.Cursor > page.Latest {
			page.Reason = "cursor_reset"
			return page, tx.Commit()
		}
		rows, err = tx.QueryContext(ctx, "SELECT cursor,payload_json FROM events WHERE cursor>? ORDER BY cursor ASC LIMIT ?", page.Cursor, limit+1)
	}
	if err != nil {
		return page, err
	}
	page.Events, err = scanSQLiteEvents(rows)
	rows.Close()
	if err != nil {
		return page, err
	}
	if options.After == nil {
		for left, right := 0, len(page.Events)-1; left < right; left, right = left+1, right-1 {
			page.Events[left], page.Events[right] = page.Events[right], page.Events[left]
		}
	} else {
		if len(page.Events) > limit {
			page.Events = page.Events[:limit]
			page.HasMore = true
		}
		if len(page.Events) > 0 {
			page.Cursor = page.Events[len(page.Events)-1].Cursor
		}
	}
	// SQLite cursor gaps caused by idempotent inserts are not retention gaps.
	filterReplay(&page, options.Filters)
	return page, tx.Commit()
}
