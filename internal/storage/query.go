package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultQueryLimit = 100
const MaxQueryLimit = 200

var ErrReadModelChanged = errors.New("read model changed; restart pagination")

type QueryValidationError struct{ Message string }

func (e *QueryValidationError) Error() string { return e.Message }
func invalidQuery(message string) error       { return &QueryValidationError{Message: message} }

type ReadModelRecord struct {
	ID        string          `json:"id"`
	UserID    int64           `json:"userId"`
	SessionID string          `json:"sessionId,omitempty"`
	IntentID  string          `json:"intentId,omitempty"`
	TurnID    string          `json:"turnId,omitempty"`
	EventType string          `json:"eventType,omitempty"`
	Status    string          `json:"status,omitempty"`
	UpdatedAt string          `json:"updatedAt"`
	Payload   json.RawMessage `json:"payload"`
}
type QueryOptions struct {
	UserID    int64  `json:"userId"`
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"-"`
}
type ReadModelPage struct {
	Kind        string            `json:"kind"`
	Records     []ReadModelRecord `json:"records"`
	Limit       int               `json:"limit"`
	NextCursor  string            `json:"nextCursor,omitempty"`
	RefreshedAt string            `json:"refreshedAt,omitempty"`
}
type queryCursor struct {
	Version   int          `json:"v"`
	Kind      string       `json:"kind"`
	Options   QueryOptions `json:"query"`
	Revision  int64        `json:"revision"`
	UpdatedAt string       `json:"updatedAt"`
	ID        string       `json:"id"`
}
type SQLiteQueryStore struct{ database *sql.DB }

func NewSQLiteQueryStore(database *sql.DB) *SQLiteQueryStore {
	return &SQLiteQueryStore{database: database}
}

// List retains the existing Go API while sharing the HTTP contract and summaries.
func (s *SQLiteQueryStore) List(ctx context.Context, kind string, userID int64, sessionID, status string, limit int) ([]ReadModelRecord, error) {
	page, err := s.Page(ctx, kind, QueryOptions{UserID: userID, SessionID: sessionID, Status: status, Limit: limit})
	return page.Records, err
}
func validateQuery(kind string, options QueryOptions) (string, QueryOptions, error) {
	table, err := queryTable(kind)
	if err != nil {
		return "", options, err
	}
	options.SessionID = strings.TrimSpace(options.SessionID)
	options.Status = strings.TrimSpace(options.Status)
	if options.UserID < 0 {
		return "", options, invalidQuery("user_id must be non-negative")
	}
	if options.Limit == 0 {
		options.Limit = DefaultQueryLimit
	}
	if options.Limit < 1 || options.Limit > MaxQueryLimit {
		return "", options, invalidQuery("limit must be between 1 and 200")
	}
	if table == "sessions" && options.Status != "" {
		return "", options, invalidQuery("status is unsupported for sessions")
	}
	return table, options, nil
}
func (s *SQLiteQueryStore) Page(ctx context.Context, kind string, options QueryOptions) (ReadModelPage, error) {
	page := ReadModelPage{Records: []ReadModelRecord{}}
	table, options, err := validateQuery(kind, options)
	if err != nil {
		return page, err
	}
	page.Kind, page.Limit = table, options.Limit
	if s == nil || s.database == nil {
		return page, fmt.Errorf("SQLite query store is not configured")
	}
	var cursor queryCursor
	if options.Cursor != "" {
		if len(options.Cursor) > 4096 {
			return page, invalidQuery("invalid cursor")
		}
		encoded, err := base64.RawURLEncoding.DecodeString(options.Cursor)
		if err != nil || json.Unmarshal(encoded, &cursor) != nil {
			return page, invalidQuery("invalid cursor")
		}
		_, timeErr := time.Parse(readModelTimeFormat, cursor.UpdatedAt)
		if cursor.Version != 1 || cursor.Kind != table || cursor.Options != (QueryOptions{UserID: options.UserID, SessionID: options.SessionID, Status: options.Status, Limit: options.Limit}) || cursor.ID == "" || timeErr != nil || cursor.Revision < 0 {
			return page, invalidQuery("cursor does not match query")
		}
	}
	tx, err := s.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	var revision int64
	err = tx.QueryRowContext(ctx, "SELECT refreshed_at, revision FROM read_model_refreshes WHERE kind = ?", table).Scan(&page.RefreshedAt, &revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return page, err
	}
	if options.Cursor != "" && cursor.Revision != revision {
		return page, ErrReadModelChanged
	}
	where, args := queryFilters(table, options)
	if options.Cursor != "" {
		where += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID)
	}
	args = append(args, options.Limit+1)
	rows, err := tx.QueryContext(ctx, "SELECT "+readModelColumns(table, false)+" FROM "+table+where+" ORDER BY updated_at DESC, id DESC LIMIT ?", args...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		record, err := scanReadModel(rows)
		if err != nil {
			rows.Close()
			return page, err
		}
		page.Records = append(page.Records, record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Records) > options.Limit {
		page.Records = page.Records[:options.Limit]
		last := page.Records[len(page.Records)-1]
		options.Cursor = ""
		encoded, err := json.Marshal(queryCursor{Version: 1, Kind: table, Options: options, Revision: revision, UpdatedAt: last.UpdatedAt, ID: last.ID})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, tx.Commit()
}
func queryFilters(table string, options QueryOptions) (string, []any) {
	where, args := " WHERE 1=1", []any{}
	if options.UserID != 0 {
		where += " AND user_id = ?"
		args = append(args, options.UserID)
	}
	if options.SessionID != "" {
		column := "session_id"
		if table == "sessions" {
			column = "id"
		}
		where += " AND " + column + " = ?"
		args = append(args, options.SessionID)
	}
	if options.Status != "" {
		where += " AND status = ?"
		args = append(args, options.Status)
	}
	return where, args
}
func readModelColumns(table string, detail bool) string {
	session, intent, turn, event, status, payload := "session_id", "''", "''", "''", "status", "payload_json"
	if table == "sessions" {
		session, status = "id", "''"
		if !detail {
			payload = "summary_json"
		}
	}
	if table == "deliveries" {
		intent, turn, event = "intent_id", "turn_id", "event_type"
	}
	return "id, user_id, " + session + ", " + intent + ", " + turn + ", " + event + ", " + status + ", updated_at, " + payload
}

type rowScanner interface{ Scan(...any) error }

func scanReadModel(row rowScanner) (ReadModelRecord, error) {
	var record ReadModelRecord
	var payload []byte
	err := row.Scan(&record.ID, &record.UserID, &record.SessionID, &record.IntentID, &record.TurnID, &record.EventType, &record.Status, &record.UpdatedAt, &payload)
	record.Payload = json.RawMessage(payload)
	return record, err
}
func (s *SQLiteQueryStore) Get(ctx context.Context, kind, id string) (ReadModelRecord, error) {
	table, err := queryTable(kind)
	if err != nil {
		return ReadModelRecord{}, err
	}
	if s == nil || s.database == nil {
		return ReadModelRecord{}, fmt.Errorf("SQLite query store is not configured")
	}
	return scanReadModel(s.database.QueryRowContext(ctx, "SELECT "+readModelColumns(table, true)+" FROM "+table+" WHERE id = ?", id))
}
func queryTable(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "sessions":
		return "sessions", nil
	case "intents":
		return "intents", nil
	case "open_loops", "open-loops":
		return "open_loops", nil
	case "deliveries":
		return "deliveries", nil
	default:
		return "", invalidQuery(fmt.Sprintf("unsupported read model %q", kind))
	}
}
