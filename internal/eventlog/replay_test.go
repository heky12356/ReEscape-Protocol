package eventlog

import (
	"context"
	"fmt"
	"testing"

	"project-yume/internal/storage"
)

func replayTestStores(t *testing.T, run func(*testing.T, ReplayStore)) {
	t.Helper()
	t.Run("file", func(t *testing.T) {
		store, err := NewFileStore(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		run(t, NewHookedStore(store, nil))
	})
	t.Run("sqlite", func(t *testing.T) {
		p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		run(t, NewHookedStore(NewSQLiteEventStore(p.Database()), nil))
	})
}

func TestReplayBaselineAndMultipleFilteredBatches(t *testing.T) {
	replayTestStores(t, func(t *testing.T, store ReplayStore) {
		ctx := context.Background()
		events := make([]Event, 80)
		for i := range events {
			events[i] = Event{ID: fmt.Sprintf("e-%d", i), Type: "probe", SessionID: "s", Tool: "legacy-tool", RequestID: "r", TurnID: "t", IntentID: "i", DeliveryID: "d"}
		}
		if err := store.Append(ctx, events...); err != nil {
			t.Fatal(err)
		}
		baseline, err := store.Replay(ctx, ReplayOptions{Limit: 10})
		if err != nil || len(baseline.Events) != 10 || baseline.Cursor != 80 || baseline.Events[0].Cursor != 71 || baseline.HasMore {
			t.Fatalf("baseline: %+v %v", baseline, err)
		}
		after := int64(0)
		total := 0
		for {
			page, err := store.Replay(ctx, ReplayOptions{After: &after, Limit: 7, Filters: Filters{Type: "probe", SessionID: "s", Tool: "legacy-tool", RequestID: "r", TurnID: "t", IntentID: "i", DeliveryID: "d"}})
			if err != nil || page.Reason != "" {
				t.Fatalf("replay: %+v %v", page, err)
			}
			for _, event := range page.Events {
				if event.Cursor <= after {
					t.Fatal("duplicate or unordered")
				}
				after = event.Cursor
				total++
			}
			after = page.Cursor
			if !page.HasMore {
				break
			}
		}
		if total != 80 || after != 80 {
			t.Fatalf("missed events: %d %d", total, after)
		}
		after = 0
		page, err := store.Replay(ctx, ReplayOptions{After: &after, Limit: 7, Filters: Filters{RequestID: "missing"}})
		if err != nil || len(page.Events) != 0 || page.Cursor != 7 || !page.HasMore {
			t.Fatalf("empty filtered scan stalled: %+v %v", page, err)
		}
		after = 999
		page, err = store.Replay(ctx, ReplayOptions{After: &after, Limit: 7})
		if err != nil || page.Reason != "cursor_reset" {
			t.Fatalf("future cursor: %+v %v", page, err)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := store.Replay(cancelled, ReplayOptions{}); err == nil {
			t.Fatal("cancelled query accepted")
		}
	})
}

func TestFileReplayRetentionGapAndBoundary(t *testing.T) {
	store, _ := NewFileStore(nil, nil)
	events := make([]Event, 1200)
	for i := range events {
		events[i] = Event{Type: "probe"}
	}
	if err := store.Append(context.Background(), events...); err != nil {
		t.Fatal(err)
	}
	after := int64(0)
	page, err := store.Replay(context.Background(), ReplayOptions{After: &after})
	if err != nil || page.Reason != "retention_gap" || page.Earliest != 201 || page.Latest != 1200 || len(page.Events) != 0 {
		t.Fatalf("gap: %+v %v", page, err)
	}
	after = 200
	page, err = store.Replay(context.Background(), ReplayOptions{After: &after, Limit: 200})
	if err != nil || page.Reason != "" || page.Events[0].Cursor != 201 || !page.HasMore {
		t.Fatalf("boundary: %+v %v", page, err)
	}
}

func TestSQLiteReplayDoesNotTreatSequenceHolesAsHistoryLoss(t *testing.T) {
	p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	store := NewSQLiteEventStore(p.Database())
	ctx := context.Background()
	event := Event{ID: "same", Type: "probe"}
	// Reusing the same timestamp is required for idempotent append.
	event = normalizeEvents([]Event{event})[0]
	if err := store.Append(ctx, event, event, Event{ID: "next", Type: "probe"}); err != nil {
		t.Fatal(err)
	}
	after := int64(1)
	page, err := store.Replay(ctx, ReplayOptions{After: &after, Limit: 20})
	if err != nil || page.Reason != "" || len(page.Events) != 1 || page.Events[0].Cursor != 3 {
		t.Fatalf("sequence hole: %+v %v", page, err)
	}
}
