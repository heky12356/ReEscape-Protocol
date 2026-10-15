package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ConversationMessage struct {
	Sequence   int             `json:"sequence"`
	Role       string          `json:"role"`
	RecordedAt *string         `json:"recordedAt"`
	Payload    json.RawMessage `json:"payload"`
}
type ConversationPage struct {
	SessionID   string                `json:"sessionId"`
	Messages    []ConversationMessage `json:"messages"`
	Limit       int                   `json:"limit"`
	NextCursor  string                `json:"nextCursor,omitempty"`
	RefreshedAt string                `json:"refreshedAt"`
}
type conversationCursor struct {
	Version   int    `json:"v"`
	SessionID string `json:"session"`
	Limit     int    `json:"limit"`
	Sequence  int    `json:"sequence"`
	Revision  int64  `json:"revision"`
	Hash      string `json:"hash"`
}

// Only canonical conversation entries are facts. InterruptedReply, intents and
// delivery drafts must never be used to reconstruct assistant text here.
func syncConversation(tx *sql.Tx, sessionID string, payload []byte) error {
	var session struct {
		Conversation []json.RawMessage `json:"conversation"`
	}
	if err := json.Unmarshal(payload, &session); err != nil {
		return err
	}
	messages := []ConversationMessage{}
	for sequence, raw := range session.Conversation {
		var metadata struct {
			Role      string `json:"role"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return err
		}
		if metadata.Role != "user" && metadata.Role != "assistant" {
			continue
		}
		message := ConversationMessage{Sequence: sequence, Role: metadata.Role, Payload: raw}
		// Standard OpenAI messages have no timestamp. Never borrow session time.
		if at, err := time.Parse(time.RFC3339Nano, metadata.CreatedAt); err == nil && !at.IsZero() {
			value := readModelTime(at)
			message.RecordedAt = &value
		}
		messages = append(messages, message)
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
	var previous string
	err = tx.QueryRow("SELECT content_hash FROM conversation_revisions WHERE session_id = ?", sessionID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && previous == hash {
		return nil
	}
	if _, err := tx.Exec("DELETE FROM conversation_messages WHERE session_id = ?", sessionID); err != nil {
		return err
	}
	for _, message := range messages {
		if _, err := tx.Exec("INSERT INTO conversation_messages(session_id,sequence,role,recorded_at,payload_json) VALUES(?,?,?,?,?)", sessionID, message.Sequence, message.Role, message.RecordedAt, string(message.Payload)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`INSERT INTO conversation_revisions(session_id,content_hash,revision,refreshed_at) VALUES(?,?,1,?)
 ON CONFLICT(session_id) DO UPDATE SET content_hash=excluded.content_hash,revision=revision+1,refreshed_at=excluded.refreshed_at`, sessionID, hash, readModelTime(time.Now()))
	return err
}

func backfillConversations(tx *sql.Tx) error {
	rows, err := tx.Query("SELECT id,payload_json FROM sessions")
	if err != nil {
		return err
	}
	type source struct {
		id      string
		payload []byte
	}
	sources := []source{}
	for rows.Next() {
		var entry source
		if err := rows.Scan(&entry.id, &entry.payload); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, entry := range sources {
		if err := syncConversation(tx, entry.id, entry.payload); err != nil {
			return fmt.Errorf("backfill conversation %s: %w", entry.id, err)
		}
	}
	return nil
}

func (s *SQLiteQueryStore) SessionSummary(ctx context.Context, id string) (ReadModelRecord, error) {
	if s == nil || s.database == nil {
		return ReadModelRecord{}, fmt.Errorf("SQLite query store is not configured")
	}
	return scanReadModel(s.database.QueryRowContext(ctx, "SELECT "+readModelColumns("sessions", false)+" FROM sessions WHERE id = ?", id))
}

// Messages reads a bounded indexed page, never the session's snapshot/payload.
// Cursors bind to this session's contents, not unrelated state flushes.
func (s *SQLiteQueryStore) Messages(ctx context.Context, sessionID string, limit int, cursor string) (ConversationPage, error) {
	page := ConversationPage{SessionID: sessionID, Messages: []ConversationMessage{}, Limit: limit}
	if limit == 0 {
		limit = DefaultQueryLimit
		page.Limit = limit
	}
	if strings.TrimSpace(sessionID) == "" || limit < 1 || limit > MaxQueryLimit {
		return page, invalidQuery("invalid session or limit (1-200)")
	}
	if s == nil || s.database == nil {
		return page, fmt.Errorf("SQLite query store is not configured")
	}
	var after conversationCursor
	if cursor != "" {
		if len(cursor) > 4096 {
			return page, invalidQuery("invalid cursor")
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(raw, &after) != nil || after.Version != 1 || after.SessionID != sessionID || after.Limit != limit || after.Sequence < 0 || after.Revision < 1 || len(after.Hash) != 64 {
			return page, invalidQuery("cursor does not match conversation query")
		}
	}
	tx, err := s.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	var hash string
	var revision int64
	err = tx.QueryRowContext(ctx, "SELECT content_hash,revision,refreshed_at FROM conversation_revisions WHERE session_id = ?", sessionID).Scan(&hash, &revision, &page.RefreshedAt)
	if err != nil {
		return page, err
	}
	if cursor != "" && (after.Revision != revision || after.Hash != hash) {
		return page, ErrReadModelChanged
	}
	sequence := -1
	if cursor != "" {
		sequence = after.Sequence
	}
	rows, err := tx.QueryContext(ctx, "SELECT sequence,role,recorded_at,payload_json FROM conversation_messages WHERE session_id = ? AND sequence > ? ORDER BY sequence ASC LIMIT ?", sessionID, sequence, limit+1)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var message ConversationMessage
		var raw []byte
		if err := rows.Scan(&message.Sequence, &message.Role, &message.RecordedAt, &raw); err != nil {
			rows.Close()
			return page, err
		}
		message.Payload = json.RawMessage(raw)
		page.Messages = append(page.Messages, message)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Messages) > limit {
		page.Messages = page.Messages[:limit]
		raw, err := json.Marshal(conversationCursor{Version: 1, SessionID: sessionID, Limit: limit, Sequence: page.Messages[limit-1].Sequence, Revision: revision, Hash: hash})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit()
}
