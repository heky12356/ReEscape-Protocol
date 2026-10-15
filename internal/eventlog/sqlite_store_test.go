package eventlog

import (
	"context"
	"testing"

	"project-yume/internal/storage"
)

func TestSQLiteStoreAppendsAndReplaysByCursor(t *testing.T) {
	persistence, err := storage.OpenPersistence(storage.PersistenceOptions{
		Backend: string(storage.BackendSQLite),
		DataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open persistence: %v", err)
	}
	defer persistence.Close()

	store := NewSQLiteEventStore(persistence.Database())
	if err := store.Append(context.Background(), Event{Type: "first"}, Event{Type: "second", UserID: 42}); err != nil {
		t.Fatalf("append events: %v", err)
	}
	events, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 || events[0].Cursor != 1 || events[1].Cursor != 2 {
		t.Fatalf("unexpected listed events: %+v", events)
	}
	if events[1].UserID != 42 {
		t.Fatalf("user id was not retained: %+v", events[1])
	}
	latest, err := store.LatestCursor(context.Background())
	if err != nil || latest != 2 {
		t.Fatalf("latest cursor = %d, %v", latest, err)
	}
	after, err := store.ListAfter(context.Background(), 1, 10)
	if err != nil || len(after) != 1 || after[0].Cursor != 2 || after[0].Type != "second" {
		t.Fatalf("unexpected replay result: %+v, %v", after, err)
	}
}

func TestFileStoreAssignsCursorsAndReplays(t *testing.T) {
	store, err := NewFileStore(nil, nil)
	if err != nil {
		t.Fatalf("create file store: %v", err)
	}
	if err := store.Append(context.Background(), Event{Type: "first"}, Event{Type: "second"}); err != nil {
		t.Fatalf("append events: %v", err)
	}
	events, err := store.ListAfter(context.Background(), 1, 10)
	if err != nil || len(events) != 1 || events[0].Cursor != 2 {
		t.Fatalf("unexpected file replay result: %+v, %v", events, err)
	}
}

func TestSQLiteStoreProjectsDeliveryRows(t *testing.T) {
	persistence, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: string(storage.BackendSQLite), DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open persistence: %v", err)
	}
	defer persistence.Close()
	store := NewSQLiteEventStore(persistence.Database())
	if err := store.Append(context.Background(), Event{Type: "delivery_sent", DeliveryID: "delivery-1", IntentID: "intent-1", UserID: 42}); err != nil {
		t.Fatalf("append delivery event: %v", err)
	}
	records, err := storage.NewSQLiteQueryStore(persistence.Database()).List(context.Background(), "deliveries", 42, "", "", 10)
	if err != nil || len(records) != 1 || records[0].ID != "delivery-1" || records[0].Status != "delivered" || records[0].EventType != "delivery_sent" {
		t.Fatalf("delivery query = %+v, %v", records, err)
	}
}
