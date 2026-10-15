package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Fixed-width UTC timestamps preserve chronological order in SQLite TEXT keys.
const readModelTimeFormat = "2006-01-02T15:04:05.000000000Z"

func readModelTime(at time.Time) string { return at.UTC().Format(readModelTimeFormat) }

func touchReadModel(ctx context.Context, tx *sql.Tx, kind string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO read_model_refreshes(kind, refreshed_at, revision) VALUES (?, ?, 1)
 ON CONFLICT(kind) DO UPDATE SET refreshed_at = excluded.refreshed_at, revision = revision + 1`, kind, readModelTime(time.Now()))
	return err
}

// ProjectDeliveryEvent consumes stored facts only. Intent and item events do not
// define an aggregate delivery outcome, even when they share a delivery ID.
func ProjectDeliveryEvent(ctx context.Context, tx *sql.Tx, cursor int64, payload []byte) error {
	var event struct {
		Type       string    `json:"type"`
		DeliveryID string    `json:"delivery_id"`
		IntentID   string    `json:"intent_id"`
		SessionID  string    `json:"session_id"`
		TurnID     string    `json:"turn_id"`
		UserID     int64     `json:"user_id"`
		CreatedAt  time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode delivery projection event: %w", err)
	}
	if event.DeliveryID == "" {
		return nil
	}
	status, terminal := "", 1
	switch event.Type {
	case "delivery_started":
		status, terminal = "sending", 0
	case "reply_delivered", "delivery_sent":
		status = "delivered"
	case "reply_partial":
		status = "partial"
	case "reply_delivery_failed":
		status = "failed"
	case "reply_cancelled":
		status = "cancelled"
	default:
		return nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO deliveries
 (id, intent_id, user_id, session_id, turn_id, status, updated_at, event_type, event_cursor, terminal, payload_json)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
 ON CONFLICT(id) DO UPDATE SET
 intent_id = COALESCE(NULLIF(excluded.intent_id, ''), deliveries.intent_id),
 user_id = COALESCE(NULLIF(excluded.user_id, 0), deliveries.user_id),
 session_id = COALESCE(NULLIF(excluded.session_id, ''), deliveries.session_id),
 turn_id = COALESCE(NULLIF(excluded.turn_id, ''), deliveries.turn_id),
 status = excluded.status, updated_at = excluded.updated_at, event_type = excluded.event_type,
 event_cursor = excluded.event_cursor, terminal = excluded.terminal, payload_json = excluded.payload_json
 WHERE excluded.terminal > deliveries.terminal OR
 (excluded.terminal = deliveries.terminal AND (excluded.updated_at > deliveries.updated_at OR
 (excluded.updated_at = deliveries.updated_at AND excluded.event_cursor > deliveries.event_cursor)))`,
		event.DeliveryID, event.IntentID, event.UserID, event.SessionID, event.TurnID, status,
		readModelTime(event.CreatedAt), event.Type, cursor, terminal, string(payload))
	if err != nil {
		return fmt.Errorf("project delivery %s: %w", event.DeliveryID, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed > 0 {
		return touchReadModel(ctx, tx, "deliveries")
	}
	return nil
}

// RebuildDeliveryReadModels rebuilds derived rows atomically without modifying
// events, invoking hooks, calling providers, or sending messages.
func RebuildDeliveryReadModels(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("SQLite database is not configured")
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := rebuildDeliveryReadModelsTx(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func rebuildDeliveryReadModelsTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM deliveries"); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT cursor, payload_json FROM events ORDER BY cursor")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cursor int64
		var payload []byte
		if err := rows.Scan(&cursor, &payload); err != nil {
			return err
		}
		if err := ProjectDeliveryEvent(ctx, tx, cursor, payload); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return touchReadModel(ctx, tx, "deliveries")
}

func sessionSummary(payload []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	summary := make(map[string]json.RawMessage)
	for _, key := range []string{"id", "user_id", "group_id", "chat_type", "current_state", "summary", "active_topics", "last_reply", "last_user_message_at", "last_assistant_message_at", "last_interaction_at", "last_proactive_at", "next_scheduled_at", "last_updated"} {
		if value, ok := fields[key]; ok {
			summary[key] = value
		}
	}
	return json.Marshal(summary)
}

func upgradeReadModelContract(tx *sql.Tx) error {
	// Upgrade prior timestamp keys and materialize summaries once, never on list.
	for _, table := range []string{"sessions", "intents", "open_loops"} {
		rows, err := tx.Query("SELECT id, updated_at, payload_json FROM " + table)
		if err != nil {
			return err
		}
		type legacyRow struct {
			id, updated string
			payload     []byte
		}
		records := []legacyRow{}
		for rows.Next() {
			var row legacyRow
			if err := rows.Scan(&row.id, &row.updated, &row.payload); err != nil {
				rows.Close()
				return err
			}
			records = append(records, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, row := range records {
			at, err := time.Parse(time.RFC3339Nano, row.updated)
			if err != nil {
				return fmt.Errorf("invalid %s timestamp: %w", table, err)
			}
			if table == "sessions" {
				summary, err := sessionSummary(row.payload)
				if err != nil {
					return err
				}
				_, err = tx.Exec("UPDATE sessions SET updated_at = ?, summary_json = ? WHERE id = ?", readModelTime(at), string(summary), row.id)
				if err != nil {
					return err
				}
			} else if _, err := tx.Exec("UPDATE "+table+" SET updated_at = ? WHERE id = ?", readModelTime(at), row.id); err != nil {
				return err
			}
		}
		if err := touchReadModel(context.Background(), tx, table); err != nil {
			return err
		}
	}
	return rebuildDeliveryReadModelsTx(context.Background(), tx)
}
