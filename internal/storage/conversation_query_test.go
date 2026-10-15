package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestConversationLifecycleAndBoundedPages(t *testing.T) {
	ctx := context.Background()
	p := queryTestPersistence(t)
	query := NewSQLiteQueryStore(p.Database())
	session := map[string]any{"user_id": 42, "conversation": []any{
		map[string]any{"role": "system", "content": "hidden"},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "hello"}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.invalid/private.png"}}}},
		map[string]any{"role": "assistant", "content": "delivered", "created_at": "2026-01-01T00:00:00Z"},
		map[string]any{"role": "user", "content": "next"},
	}, "interrupted_reply": map[string]any{"status": "cancelled", "delivered_segments": []string{"delivered"}, "undelivered_segments": []string{"undelivered secret"}}}
	sessions := map[string]any{"s": session}
	save := func() {
		t.Helper()
		data, err := json.Marshal(map[string]any{"sessions": sessions})
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
			t.Fatal(err)
		}
	}
	save()
	first, err := query.Messages(ctx, "s", 1, "")
	if err != nil || len(first.Messages) != 1 || first.NextCursor == "" {
		t.Fatalf("first: %+v %v", first, err)
	}
	if first.Messages[0].Sequence != 1 || first.Messages[0].RecordedAt != nil {
		t.Fatal("wrong ordinal or fabricated time")
	}
	// Unchanged save/load and a different session must not invalidate this cursor.
	sessions["other"] = map[string]any{"conversation": []any{map[string]any{"role": "user", "content": "unrelated"}}}
	save()
	save()
	if _, err := p.Snapshots.Load("memory/conversation_history.json"); err != nil {
		t.Fatal(err)
	}
	second, err := query.Messages(ctx, "s", 1, first.NextCursor)
	if err != nil || second.Messages[0].Role != "assistant" || second.Messages[0].RecordedAt == nil {
		t.Fatalf("second: %+v %v", second, err)
	}
	third, err := query.Messages(ctx, "s", 1, second.NextCursor)
	if err != nil || third.NextCursor != "" || third.Messages[0].Sequence != 3 {
		t.Fatalf("third: %+v %v", third, err)
	}
	var count int
	if err := p.Database().QueryRow("SELECT COUNT(*) FROM conversation_messages WHERE session_id='s'").Scan(&count); err != nil || count != 3 {
		t.Fatalf("duplicate messages: %d %v", count, err)
	}
	if _, err := query.Messages(ctx, "other", 1, first.NextCursor); err == nil {
		t.Fatal("cursor crossed session")
	}
	if _, err := query.Messages(ctx, "s", 2, first.NextCursor); err == nil {
		t.Fatal("cursor crossed limit")
	}
	for _, limit := range []int{-1, 201} {
		if _, err := query.Messages(ctx, "s", limit, ""); err == nil {
			t.Fatal("invalid limit")
		}
	}
	// A message page and session summary do not deserialize the full session.
	if _, err := p.Database().Exec("UPDATE sessions SET payload_json='{' WHERE id='s'"); err != nil {
		t.Fatal(err)
	}
	if _, err := query.SessionSummary(ctx, "s"); err != nil {
		t.Fatal(err)
	}
	if _, err := query.Messages(ctx, "s", 1, first.NextCursor); err != nil {
		t.Fatal(err)
	}
	session["conversation"] = []any{map[string]any{"role": "assistant", "content": "new fact"}}
	save()
	if _, err := query.Messages(ctx, "s", 1, first.NextCursor); !errors.Is(err, ErrReadModelChanged) {
		t.Fatalf("stale: %v", err)
	}
	delete(sessions, "s")
	save()
	if _, err := query.Messages(ctx, "s", 1, ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted session: %v", err)
	}
	if err := p.Database().QueryRow("SELECT COUNT(*) FROM conversation_messages WHERE session_id='s'").Scan(&count); err != nil || count != 0 {
		t.Fatal("orphan messages")
	}
}

func TestConversationMigrationBackfillsExistingSessions(t *testing.T) {
	p := queryTestPersistence(t)
	db := p.Database()
	if err := p.Snapshots.Save("memory/conversation_history.json", []byte(`{"sessions":{"legacy":{"conversation":[{"role":"user","content":"old history"}]}}}`)); err != nil {
		t.Fatal(err)
	}
	// Recreate precisely the pre-v4 schema. Existing facts must survive upgrade.
	for _, statement := range []string{"DROP TABLE conversation_messages", "DROP TABLE conversation_revisions", "DELETE FROM schema_migrations WHERE version=4"} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateSQLite(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateSQLite(db); err != nil {
		t.Fatal(err)
	}
	page, err := NewSQLiteQueryStore(db).Messages(context.Background(), "legacy", 10, "")
	if err != nil || len(page.Messages) != 1 || !strings.Contains(string(page.Messages[0].Payload), "old history") {
		t.Fatalf("migration: %+v %v", page, err)
	}
}

func TestLargeConversationPageUsesIndex(t *testing.T) {
	p := queryTestPersistence(t)
	messages := make([]any, 10000)
	for i := range messages {
		messages[i] = map[string]any{"role": "user", "content": fmt.Sprintf("message %d", i)}
	}
	data, _ := json.Marshal(map[string]any{"sessions": map[string]any{"large": map[string]any{"conversation": messages}}})
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	page, err := NewSQLiteQueryStore(p.Database()).Messages(context.Background(), "large", 50, "")
	if err != nil || len(page.Messages) != 50 || page.NextCursor == "" {
		t.Fatalf("large page: %+v %v", page, err)
	}
	var id, parent, unused int
	var plan string
	err = p.Database().QueryRow("EXPLAIN QUERY PLAN SELECT sequence,role,recorded_at,payload_json FROM conversation_messages WHERE session_id=? AND sequence>? ORDER BY sequence ASC LIMIT ?", "large", 5000, 51).Scan(&id, &parent, &unused, &plan)
	if err != nil || !strings.Contains(plan, "INDEX") || !strings.Contains(plan, "session_id=? AND sequence>?") {
		t.Fatalf("unindexed plan: %s %v", plan, err)
	}
	t.Logf("10,000 messages, page=50; query plan: %s", plan)
}

func TestConversationMigrationFailureRollsBack(t *testing.T) {
	p := queryTestPersistence(t)
	db := p.Database()
	for _, statement := range []string{
		"DROP TABLE conversation_messages", "DROP TABLE conversation_revisions",
		"DELETE FROM schema_migrations WHERE version=4",
		`INSERT INTO sessions(id,user_id,updated_at,payload_json,summary_json) VALUES('bad',42,'2026-01-01T00:00:00Z','{"conversation":"invalid"}','{}')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateSQLite(db); err == nil {
		t.Fatal("invalid legacy conversation accepted")
	}
	var version, tables int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil || version != 3 {
		t.Fatalf("migration advanced after failure: %d %v", version, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name IN ('conversation_messages','conversation_revisions')").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("partial migration leaked: %d %v", tables, err)
	}
	var original string
	if err := db.QueryRow("SELECT payload_json FROM sessions WHERE id='bad'").Scan(&original); err != nil || original != `{"conversation":"invalid"}` {
		t.Fatal("legacy fact changed", err)
	}
}
