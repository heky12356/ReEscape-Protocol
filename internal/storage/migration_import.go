package storage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var migrationSnapshotNames = []string{
	"memory/conversation_history.json", "memory/user_profiles.json", "memory/fact_memories.json",
	"memory/emotional_memory.json", "affection/affection_state.json", "events/react_event_log.json",
}

// MigrateJSONOffline builds a new database privately, preserves source files,
// and refuses even an empty existing destination. Repeating it never duplicates facts.
func MigrateJSONOffline(ctx context.Context, options PersistenceOptions) (BackupReport, error) {
	options = maintenanceOptions(options)
	options.Backend = "sqlite"
	lease, err := acquirePersistenceLease(options)
	if err != nil {
		return BackupReport{}, err
	}
	defer lease.Close()
	if err := recoverPendingFileRestores(options); err != nil {
		return BackupReport{}, err
	}
	if err := rejectSymlinks(options.SQLitePath); err != nil {
		return BackupReport{}, err
	}
	if _, err := os.Lstat(options.SQLitePath); !os.IsNotExist(err) {
		return BackupReport{}, fmt.Errorf("migration target must not exist; source files were not changed")
	}
	if err := requireNoSQLiteSidecars(options.SQLitePath); err != nil {
		return BackupReport{}, err
	}
	documents := map[string][]byte{}
	source := NewFileSnapshotStore(options.DataDir)
	for _, name := range migrationSnapshotNames {
		for _, suffix := range []string{"", ".bak"} {
			full := filepath.Join(options.DataDir, filepath.FromSlash(name)) + suffix
			if err := rejectSymlinks(full); err != nil {
				return BackupReport{}, err
			}
			if info, err := os.Stat(full); err == nil && (!info.Mode().IsRegular() || info.Size() > MaxTransferArchiveBytes) {
				return BackupReport{}, fmt.Errorf("invalid or oversized JSON source: %s", name)
			} else if err != nil && !os.IsNotExist(err) {
				return BackupReport{}, err
			}
		}
		if err := ctx.Err(); err != nil {
			return BackupReport{}, err
		}
		data, err := source.Load(name)
		if err != nil {
			return BackupReport{}, err
		}
		if len(data) == 0 {
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil || object == nil {
			return BackupReport{}, fmt.Errorf("snapshot must contain an object: %s", name)
		}
		if name == "memory/conversation_history.json" {
			data, err = normalizeMigrationSessions(data)
			if err != nil {
				return BackupReport{}, err
			}
		}
		documents[name] = data
	}
	if len(documents) == 0 {
		return BackupReport{}, fmt.Errorf("no known snapshots found; no database was created")
	}
	temp, err := os.CreateTemp(filepath.Dir(options.SQLitePath), ".json-migration-*")
	if err != nil {
		return BackupReport{}, err
	}
	staged := temp.Name()
	temp.Close()
	defer func() {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			os.Remove(staged + suffix)
		}
	}()
	db, err := openSQLiteDatabase(staged, options.BusyTimeout)
	if err != nil {
		return BackupReport{}, err
	}
	defer db.Close()
	if err := migrateSQLite(db); err != nil {
		return BackupReport{}, err
	}
	store := NewSQLiteSnapshotStore(db)
	for _, name := range migrationSnapshotNames {
		data, ok := documents[name]
		if !ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return BackupReport{}, err
		}
		if err := store.Save(name, data); err != nil {
			return BackupReport{}, err
		}
	}
	if events := documents["events/react_event_log.json"]; len(events) > 0 {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return BackupReport{}, err
		}
		defer tx.Rollback()
		var snapshot struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal(events, &snapshot); err != nil {
			return BackupReport{}, err
		}
		var previous int64
		for index, raw := range snapshot.Events {
			var event struct {
				ID         string    `json:"id"`
				Type       string    `json:"type"`
				CreatedAt  time.Time `json:"created_at"`
				UserID     int64     `json:"user_id"`
				SessionID  string    `json:"session_id"`
				TurnID     string    `json:"turn_id"`
				IntentID   string    `json:"intent_id"`
				DeliveryID string    `json:"delivery_id"`
				Cursor     int64     `json:"cursor"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				return BackupReport{}, err
			}
			if event.Type == "" {
				return BackupReport{}, fmt.Errorf("event %d has no type", index)
			}
			if event.ID == "" {
				event.ID = fmt.Sprintf("import-%x", sha256.Sum256(append([]byte(strconv.Itoa(index)+":"), raw...)))
			}
			if event.Cursor <= previous {
				event.Cursor = previous + 1
			}
			previous = event.Cursor
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				return BackupReport{}, err
			}
			fields["id"], _ = json.Marshal(event.ID)
			fields["cursor"], _ = json.Marshal(event.Cursor)
			payload, err := json.Marshal(fields)
			if err != nil {
				return BackupReport{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO events(cursor,id,type,created_at,user_id,session_id,turn_id,intent_id,delivery_id,payload_json) VALUES(?,?,?,?,?,?,?,?,?,?)`, event.Cursor, event.ID, event.Type, readModelTime(event.CreatedAt), event.UserID, event.SessionID, event.TurnID, event.IntentID, event.DeliveryID, string(payload)); err != nil {
				return BackupReport{}, fmt.Errorf("import event %d: %w", index, err)
			}
			if err := ProjectDeliveryEvent(ctx, tx, event.Cursor, payload); err != nil {
				return BackupReport{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return BackupReport{}, err
		}
	}
	if err := db.Close(); err != nil {
		return BackupReport{}, err
	}
	if err := syncFile(staged); err != nil {
		return BackupReport{}, err
	}
	report, err := ValidateSQLiteBackup(ctx, staged)
	if err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := os.Link(staged, options.SQLitePath); err != nil {
		return report, fmt.Errorf("publish migrated database: %w", err)
	}
	return report, syncDir(filepath.Dir(options.SQLitePath))
}

func normalizeMigrationSessions(data []byte) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if _, ok := envelope["sessions"]; ok {
		return data, nil
	}
	raw, ok := envelope["conversations"]
	if !ok {
		return nil, fmt.Errorf("unknown conversation snapshot format")
	}
	var conversations map[string]json.RawMessage
	if err := json.Unmarshal(raw, &conversations); err != nil {
		return nil, err
	}
	sessions := map[string]json.RawMessage{}
	for key, raw := range conversations {
		if string(raw) == "null" {
			continue
		}
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid legacy user id")
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		sessionID := "private:" + strconv.FormatInt(id, 10)
		record["id"], _ = json.Marshal(sessionID)
		record["user_id"], _ = json.Marshal(id)
		record["chat_type"] = json.RawMessage("1")
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		sessions[sessionID] = encoded
	}
	encoded, err := json.Marshal(sessions)
	if err != nil {
		return nil, err
	}
	envelope["sessions"] = encoded
	delete(envelope, "conversations")
	return json.Marshal(envelope)
}
