package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func queryTestPersistence(t *testing.T) *Persistence {
	t.Helper()
	p, err := OpenPersistence(PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func TestReadModelPaginationAndSessionSummary(t *testing.T) {
	ctx := context.Background()
	p := queryTestPersistence(t)
	snapshot := []byte(`{"sessions":{"a":{"user_id":42,"last_updated":"2026-01-01T00:00:00Z","conversation":[{"role":"user","content":"private body"}]},"b":{"user_id":42,"last_updated":"2026-01-01T00:00:00Z"},"c":{"user_id":42,"last_updated":"2026-01-01T00:00:00.9Z"},"other":{"user_id":99,"last_updated":"2026-01-02T00:00:00Z"}}}`)
	if err := p.Snapshots.Save("memory/conversation_history.json", snapshot); err != nil {
		t.Fatal(err)
	}
	query := NewSQLiteQueryStore(p.Database())
	options := QueryOptions{UserID: 42, Limit: 1}
	ids := []string{}
	for {
		page, err := query.Page(ctx, "sessions", options)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Records) != 1 || page.RefreshedAt == "" {
			t.Fatalf("page: %+v", page)
		}
		if strings.Contains(string(page.Records[0].Payload), "conversation") || strings.Contains(string(page.Records[0].Payload), "private body") {
			t.Fatal("list exposed conversation")
		}
		ids = append(ids, page.Records[0].ID)
		if page.NextCursor == "" {
			break
		}
		options.Cursor = page.NextCursor
		if len(ids) > 4 {
			t.Fatal("pagination did not terminate")
		}
	}
	if !reflect.DeepEqual(ids, []string{"c", "b", "a"}) {
		t.Fatalf("page order = %v", ids)
	}
	detail, err := query.Get(ctx, "sessions", "a")
	if err != nil || !strings.Contains(string(detail.Payload), "private body") {
		t.Fatalf("detail: %+v, %v", detail, err)
	}
	filtered, err := query.Page(ctx, "sessions", QueryOptions{UserID: 42, SessionID: "a"})
	if err != nil || len(filtered.Records) != 1 || filtered.Records[0].SessionID != "a" {
		t.Fatalf("session filter: %+v %v", filtered, err)
	}
	// The list must use its stored summary even if the full payload is unreadable.
	if _, err := p.Database().Exec("UPDATE sessions SET payload_json = '{' WHERE id = 'a'"); err != nil {
		t.Fatal(err)
	}
	if _, err := query.Page(ctx, "sessions", QueryOptions{SessionID: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := query.Get(ctx, "sessions", "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing detail: %v", err)
	}
}

func TestReadModelCursorValidationAndUpdate(t *testing.T) {
	ctx := context.Background()
	p := queryTestPersistence(t)
	data := []byte(`{"intents":{"a":{"user_id":42,"session_id":"s","status":"pending","updated_at":"2026-01-01T00:00:00Z"},"b":{"user_id":42,"session_id":"s","status":"pending","updated_at":"2026-01-01T00:00:00Z"}}}`)
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	q := NewSQLiteQueryStore(p.Database())
	options := QueryOptions{UserID: 42, SessionID: "s", Status: "pending", Limit: 1}
	page, err := q.Page(ctx, "intents", options)
	if err != nil || page.NextCursor == "" {
		t.Fatalf("first page: %+v %v", page, err)
	}
	options.Cursor = page.NextCursor
	for _, change := range []QueryOptions{
		{UserID: 99, SessionID: "s", Status: "pending", Limit: 1, Cursor: options.Cursor},
		{UserID: 42, SessionID: "other", Status: "pending", Limit: 1, Cursor: options.Cursor},
		{UserID: 42, SessionID: "s", Status: "completed", Limit: 1, Cursor: options.Cursor},
		{UserID: 42, SessionID: "s", Status: "pending", Limit: 2, Cursor: options.Cursor},
		{Limit: 1, Cursor: "not-a-cursor"},
		{Limit: 201}, {Limit: -1}, {UserID: -1},
	} {
		_, err := q.Page(ctx, "intents", change)
		var invalid *QueryValidationError
		if !errors.As(err, &invalid) {
			t.Fatalf("accepted invalid query %+v: %v", change, err)
		}
	}
	if _, err := q.Page(ctx, "open_loops", options); err == nil {
		t.Fatal("accepted cross-kind cursor")
	}
	if _, err := q.Page(ctx, "sessions", QueryOptions{Status: "pending"}); err == nil {
		t.Fatal("accepted unsupported filter")
	}
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Page(ctx, "intents", options); !errors.Is(err, ErrReadModelChanged) {
		t.Fatalf("stale cursor: %v", err)
	}
	empty, err := q.Page(ctx, "intents", QueryOptions{UserID: 999, Limit: 200})
	if err != nil || empty.Records == nil || len(empty.Records) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty page: %+v %v", empty, err)
	}
}

func TestReadModelMigrationRepairsLegacyDeliveryAndSummaries(t *testing.T) {
	db, err := openSQLiteDatabase(t.TempDir()+"/legacy.sqlite", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT,applied_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	for version, name := range []string{"0001_initial.sql", "0002_event_user_id.sql"} {
		script, err := sqliteMigrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(script)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO schema_migrations VALUES (?, ?, '')", version+1, name); err != nil {
			t.Fatal(err)
		}
	}
	payload := `{"user_id":42,"conversation":[{"content":"old private body"}],"summary":"short"}`
	if _, err := db.Exec("INSERT INTO sessions VALUES ('s',42,'2026-01-01T00:00:00Z',?)", payload); err != nil {
		t.Fatal(err)
	}
	for i, eventType := range []string{"delivery_started", "reply_partial", "intent_delivery_deferred"} {
		event := fmt.Sprintf(`{"id":"e%d","type":"%s","delivery_id":"d","intent_id":"i","session_id":"s","turn_id":"t","user_id":42,"created_at":"2026-01-01T00:00:0%dZ"}`, i, eventType, i)
		if _, err := db.Exec("INSERT INTO events(id,type,created_at,payload_json) VALUES (?, ?, '', ?)", fmt.Sprint(i), eventType, event); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO deliveries VALUES ('d','i',42,'intent_delivery_deferred','2026-01-01T00:00:02Z','{}')"); err != nil {
		t.Fatal(err)
	}
	if err := migrateSQLite(db); err != nil {
		t.Fatal(err)
	}
	q := NewSQLiteQueryStore(db)
	record, err := q.Get(context.Background(), "deliveries", "d")
	if err != nil || record.Status != "partial" || record.EventType != "reply_partial" || record.SessionID != "s" || record.TurnID != "t" {
		t.Fatalf("upgraded delivery: %+v %v", record, err)
	}
	page, err := q.Page(context.Background(), "sessions", QueryOptions{})
	if err != nil || len(page.Records) != 1 || strings.Contains(string(page.Records[0].Payload), "old private body") {
		t.Fatalf("upgraded summary: %+v %v", page, err)
	}
	var fields map[string]any
	if err := json.Unmarshal(page.Records[0].Payload, &fields); err != nil || fields["summary"] != "short" {
		t.Fatalf("summary: %v %v", fields, err)
	}
	if err := migrateSQLite(db); err != nil {
		t.Fatal(err)
	}
	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventCount); err != nil || eventCount != 3 {
		t.Fatalf("events modified: %d %v", eventCount, err)
	}
}

func TestPersistenceInfoReflectsLiveInstance(t *testing.T) {
	ctx := context.Background()
	p := queryTestPersistence(t)
	info, err := p.Info(ctx)
	if err != nil || info.Backend != BackendSQLite || info.SchemaVersion != 4 || len(info.ReadModels) != 4 || !info.Ready {
		t.Fatalf("info: %+v %v", info, err)
	}
	file, err := OpenPersistence(PersistenceOptions{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// Opening another backend cannot replace the first instance's queries.
	info, err = p.Info(ctx)
	if err != nil || info.Backend != BackendSQLite {
		t.Fatalf("instance lost: %+v %v", info, err)
	}
	fileInfo, err := file.Info(ctx)
	if err != nil || fileInfo.Backend != BackendFile || len(fileInfo.ReadModels) != 0 || fileInfo.SchemaVersion != 0 {
		t.Fatalf("file info: %+v %v", fileInfo, err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	closed, err := p.Info(ctx)
	if err != nil || closed.Ready {
		t.Fatalf("closed instance ready: %+v %v", closed, err)
	}
}
