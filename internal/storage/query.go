package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type ReadModelRecord struct {
	ID        string          `json:"id"`
	UserID    int64           `json:"userId"`
	SessionID string          `json:"sessionId,omitempty"`
	Status    string          `json:"status,omitempty"`
	UpdatedAt string          `json:"updatedAt"`
	Payload   json.RawMessage `json:"payload"`
}

type SQLiteQueryStore struct {
	database *sql.DB
}

func NewSQLiteQueryStore(database *sql.DB) *SQLiteQueryStore {
	return &SQLiteQueryStore{database: database}
}

func (s *SQLiteQueryStore) List(ctx context.Context, kind string, userID int64, sessionID, status string, limit int) ([]ReadModelRecord, error) {
	if s == nil || s.database == nil {
		return nil, fmt.Errorf("SQLite query store is not configured")
	}
	table, err := queryTable(kind)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	sessionColumn := "session_id"
	statusColumn := "status"
	if table == "sessions" || table == "deliveries" {
		sessionColumn = "''"
	}
	if table == "sessions" {
		statusColumn = "''"
	}
	query := fmt.Sprintf("SELECT id, user_id, %s, %s, updated_at, payload_json FROM %s WHERE 1=1", sessionColumn, statusColumn, table)
	args := make([]any, 0, 4)
	if userID != 0 {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	if sessionID != "" && (table == "intents" || table == "open_loops") {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	if status != "" && table != "sessions" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query SQLite %s: %w", kind, err)
	}
	defer rows.Close()
	result := make([]ReadModelRecord, 0)
	for rows.Next() {
		var record ReadModelRecord
		var payload string
		if err := rows.Scan(&record.ID, &record.UserID, &record.SessionID, &record.Status, &record.UpdatedAt, &payload); err != nil {
			return nil, fmt.Errorf("scan SQLite %s: %w", kind, err)
		}
		record.Payload = json.RawMessage(payload)
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite %s: %w", kind, err)
	}
	return result, nil
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
		return "", fmt.Errorf("unsupported read model %q", kind)
	}
}
