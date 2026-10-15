package eventlog

import (
	"context"
	"reflect"
	"testing"
	"time"

	"project-yume/internal/storage"
)

func TestDeliveryProjectionIgnoresIntentItemsAndLateStarts(t *testing.T) {
	p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	store := NewSQLiteEventStore(p.Database())
	ctx := context.Background()
	at := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	sequence := []Event{
		{ID: "start", Type: "delivery_started", DeliveryID: "d", IntentID: "i", SessionID: "s", TurnID: "t", UserID: 42, CreatedAt: at},
		{ID: "item", Type: "delivery_item_delivered", DeliveryID: "d", CreatedAt: at.Add(time.Second)},
		{ID: "final", Type: "reply_partial", DeliveryID: "d", IntentID: "i", SessionID: "s", TurnID: "t", UserID: 42, CreatedAt: at.Add(2 * time.Second)},
		{ID: "intent", Type: "intent_delivery_deferred", DeliveryID: "d", CreatedAt: at.Add(3 * time.Second)},
		{ID: "late-start", Type: "delivery_started", DeliveryID: "d", CreatedAt: at.Add(4 * time.Second)},
		{ID: "older-final", Type: "reply_delivery_failed", DeliveryID: "d", CreatedAt: at.Add(time.Second)},
	}
	if err := store.Append(ctx, sequence...); err != nil {
		t.Fatal(err)
	}
	q := storage.NewSQLiteQueryStore(p.Database())
	record, err := q.Get(ctx, "deliveries", "d")
	if err != nil || record.Status != "partial" || record.EventType != "reply_partial" || record.SessionID != "s" || record.TurnID != "t" || record.IntentID != "i" {
		t.Fatalf("projection: %+v %v", record, err)
	}
	filtered, err := q.List(ctx, "deliveries", 42, "s", "partial", 200)
	if err != nil || len(filtered) != 1 {
		t.Fatalf("delivery filters: %+v %v", filtered, err)
	}
	missing, err := q.List(ctx, "deliveries", 42, "other", "partial", 1)
	if err != nil || len(missing) != 0 {
		t.Fatalf("ignored session filter: %+v %v", missing, err)
	}
	before, err := store.List(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RebuildDeliveryReadModels(ctx, p.Database()); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := q.Get(ctx, "deliveries", "d")
	if err != nil || !reflect.DeepEqual(rebuilt, record) {
		t.Fatalf("rebuild changed outcome: %+v %v", rebuilt, err)
	}
	after, err := store.List(ctx, 100)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("rebuild mutated event facts")
	}
}

func TestDeliveryProjectionDuplicateConflictAndRebuildRollback(t *testing.T) {
	p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	store := NewSQLiteEventStore(p.Database())
	ctx := context.Background()
	event := Event{ID: "one", Type: "reply_delivered", DeliveryID: "d", CreatedAt: time.Now(), Data: map[string]any{"message_id": int64(9007199254740993)}}
	if err := store.Append(ctx, event, event); err != nil {
		t.Fatal(err)
	}
	events, err := store.List(ctx, 100)
	if err != nil || len(events) != 1 {
		t.Fatalf("duplicate facts: %v %v", events, err)
	}
	original := events[0]
	// Re-appending an event read from the store ignores its assigned cursor.
	if err := store.Append(ctx, original); err != nil {
		t.Fatal(err)
	}
	conflict := event
	conflict.Type = "reply_cancelled"
	if err := store.Append(ctx, Event{ID: "must-rollback", Type: "reply_partial", DeliveryID: "x"}, conflict); err == nil {
		t.Fatal("conflicting ID accepted")
	}
	events, err = store.List(ctx, 100)
	if err != nil || len(events) != 1 {
		t.Fatalf("failed append was not atomic: %v %v", events, err)
	}
	q := storage.NewSQLiteQueryStore(p.Database())
	before, err := q.Get(ctx, "deliveries", "d")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Database().Exec("INSERT INTO events(id,type,created_at,payload_json) VALUES ('bad','bad','','{')"); err != nil {
		t.Fatal(err)
	}
	if err := storage.RebuildDeliveryReadModels(ctx, p.Database()); err == nil {
		t.Fatal("corrupt fact accepted")
	}
	after, err := q.Get(ctx, "deliveries", "d")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed rebuild damaged projection")
	}
}

func TestDeliveryProjectionAllTerminalStatusesAndEqualTime(t *testing.T) {
	p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	store := NewSQLiteEventStore(p.Database())
	ctx := context.Background()
	at := time.Now()
	for _, example := range []struct{ kind, status string }{
		{"reply_delivered", "delivered"}, {"reply_partial", "partial"}, {"reply_delivery_failed", "failed"}, {"reply_cancelled", "cancelled"},
	} {
		if err := store.Append(ctx, Event{ID: example.kind, Type: example.kind, DeliveryID: example.kind, CreatedAt: at}, Event{Type: "intent_delivery_completed", DeliveryID: example.kind, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		record, err := storage.NewSQLiteQueryStore(p.Database()).Get(ctx, "deliveries", example.kind)
		if err != nil || record.Status != example.status {
			t.Fatalf("%s -> %+v %v", example.kind, record, err)
		}
	}
	if err := store.Append(ctx, Event{ID: "tie-a", Type: "reply_partial", DeliveryID: "tie", CreatedAt: at}, Event{ID: "tie-b", Type: "reply_cancelled", DeliveryID: "tie", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	record, err := storage.NewSQLiteQueryStore(p.Database()).Get(ctx, "deliveries", "tie")
	if err != nil || record.Status != "cancelled" {
		t.Fatalf("equal timestamp order: %+v %v", record, err)
	}
}
