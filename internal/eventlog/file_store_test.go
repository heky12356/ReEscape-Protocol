package eventlog

import (
	"context"
	"testing"

	"project-yume/internal/storage"
)

func TestFileStoreAppendListAndFlushRoundTrip(t *testing.T) {
	store := storage.NewFileSnapshotStore(t.TempDir())
	fs, err := NewFileStore(store, nil)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := fs.Append(context.Background(), Event{}, Event{Type: "message", RequestID: "req-1", TurnID: "turn-1", DeliveryID: "delivery-1", IntentID: "intent-1", SessionID: "s1", ToolName: "search", Model: "model-a"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := fs.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	reloaded, err := NewFileStore(store, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	events, err := reloaded.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 || events[0].Type != "message" || events[0].ID == "" || events[0].RequestID != "req-1" || events[0].TurnID != "turn-1" || events[0].DeliveryID != "delivery-1" || events[0].IntentID != "intent-1" || events[0].ToolName != "search" || events[0].Model != "model-a" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestFileStoreHonorsCanceledContext(t *testing.T) {
	fs, err := NewFileStore(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := fs.Append(ctx, Event{Type: "message"}); err == nil {
		t.Fatal("Append should return context cancellation")
	}
	if _, err := fs.List(ctx, 1); err == nil {
		t.Fatal("List should return context cancellation")
	}
}
