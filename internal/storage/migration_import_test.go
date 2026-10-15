package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeMigrationSource(t *testing.T, root, name, data string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestJSONMigrationPreservesSnapshotsFactsAndSource(t *testing.T) {
	root := t.TempDir()
	documents := map[string]string{
		"memory/conversation_history.json": `{"sessions":{"private:42":{"user_id":42,"conversation":[{"content":"history"}],"last_updated":"2026-01-01T00:00:00Z"}},"intents":{"i":{"user_id":42,"session_id":"private:42","status":"pending"}},"open_loops":{"o":{"user_id":42,"session_id":"private:42","status":"open"}}}`,
		"memory/user_profiles.json":        `{"42":{"user_id":42,"likes":["tea"]}}`,
		"memory/fact_memories.json":        `{"42":[{"id":"f","summary":"tea"}]}`,
		"memory/emotional_memory.json":     `{"42":{"user_id":42,"interactions":[]}}`,
		"affection/affection_state.json":   `{"states":{}}`,
		"events/react_event_log.json":      `{"events":[{"id":"a","cursor":18,"type":"reply_partial","delivery_id":"d","user_id":42,"session_id":"private:42","created_at":"2026-01-01T00:00:00Z","data":{"message_id":9007199254740993}},{"id":"b","cursor":19,"type":"intent_delivery_deferred","delivery_id":"d","created_at":"2026-01-01T00:00:01Z"}]}`,
	}
	for name, data := range documents {
		writeMigrationSource(t, root, name, data)
	}
	options := PersistenceOptions{DataDir: root, SQLitePath: filepath.Join(t.TempDir(), "migrated.db")}
	report, err := MigrateJSONOffline(context.Background(), options)
	if err != nil || report.Snapshots != 6 || report.Events != 2 {
		t.Fatalf("migration: %+v %v", report, err)
	}
	for name, data := range documents {
		after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(after) != data {
			t.Fatalf("source changed: %s %v", name, err)
		}
	}
	if _, err := MigrateJSONOffline(context.Background(), options); err == nil {
		t.Fatal("repeated migration accepted")
	}
	options.Backend = "sqlite"
	p, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	query := NewSQLiteQueryStore(p.Database())
	for _, kind := range []string{"sessions", "intents", "open_loops", "deliveries"} {
		records, err := query.List(context.Background(), kind, 42, "private:42", "", 10)
		if err != nil || len(records) != 1 {
			t.Fatalf("query %s: %+v %v", kind, records, err)
		}
	}
	delivery, err := query.Get(context.Background(), "deliveries", "d")
	if err != nil || delivery.Status != "partial" {
		t.Fatalf("delivery: %+v %v", delivery, err)
	}
	var cursor int64
	if err := p.Database().QueryRow("SELECT MAX(cursor) FROM events").Scan(&cursor); err != nil || cursor != 19 {
		t.Fatalf("cursor: %d %v", cursor, err)
	}
	if _, err := MigrateJSONOffline(context.Background(), options); !errors.Is(err, ErrPersistenceBusy) {
		t.Fatalf("live migration accepted: %v", err)
	}
}

func TestJSONMigrationLegacyAndFailuresDoNotPublish(t *testing.T) {
	root := t.TempDir()
	options := PersistenceOptions{DataDir: root, SQLitePath: filepath.Join(t.TempDir(), "new.sqlite")}
	writeMigrationSource(t, root, "memory/conversation_history.json", `{"conversations":{"42":{"conversation":[{"content":"legacy"}],"last_updated":"2026-01-01T00:00:00Z"}}}`)
	writeMigrationSource(t, root, "events/react_event_log.json", `{"events":[{"id":"duplicate","type":"x"},{"id":"duplicate","type":"y"}]}`)
	if _, err := MigrateJSONOffline(context.Background(), options); err == nil {
		t.Fatal("duplicate source event ID accepted")
	}
	if _, err := os.Stat(options.SQLitePath); !os.IsNotExist(err) {
		t.Fatal("failed migration published database")
	}
	writeMigrationSource(t, root, "events/react_event_log.json", `{"events":[{"type":"legacy"}]}`)
	if _, err := MigrateJSONOffline(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	options.Backend = "sqlite"
	p, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	record, err := NewSQLiteQueryStore(p.Database()).Get(context.Background(), "sessions", "private:42")
	if err != nil || record.UserID != 42 {
		t.Fatalf("legacy session: %+v %v", record, err)
	}
}
