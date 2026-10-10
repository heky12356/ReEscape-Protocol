package eventlog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventCorrelationRoundTripAndLegacyTool(t *testing.T) {
	legacy := `{"id":"evt-1","type":"tool_called","tool":"search","created_at":"2026-01-01T00:00:00Z"}`
	var event Event
	if err := json.Unmarshal([]byte(legacy), &event); err != nil {
		t.Fatal(err)
	}
	event = event.WithCorrelation(Correlation{RequestID: "req-1", TurnID: "turn-1", Model: "model-a"})
	if event.ToolName != "search" || event.Tool != "search" || event.TurnID != "turn-1" {
		t.Fatalf("legacy event was not enriched: %#v", event)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"request_id":"req-1"`) || !strings.Contains(string(encoded), `"model":"model-a"`) {
		t.Fatalf("correlation fields missing: %s", encoded)
	}
}

func TestHookedStoreBlockingHookCanRejectAppend(t *testing.T) {
	base, err := NewFileStore(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewDispatcher(HookOptions{})
	dispatcher.AddBlocking(func(context.Context, Event) error { return errors.New("rejected") })
	store := NewHookedStore(base, dispatcher)
	if err := store.Append(context.Background(), Event{Type: "blocked"}); err == nil {
		t.Fatal("expected blocking hook error")
	}
	events, err := base.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("rejected event was persisted: %#v", events)
	}
}

func TestHookedStoreBlockingHookTimeout(t *testing.T) {
	base, _ := NewFileStore(nil, nil)
	dispatcher := NewDispatcher(HookOptions{Timeout: 20 * time.Millisecond})
	dispatcher.AddBlocking(func(ctx context.Context, _ Event) error {
		<-ctx.Done()
		return ctx.Err()
	})
	started := time.Now()
	err := NewHookedStore(base, dispatcher).Append(context.Background(), Event{Type: "timeout"})
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("blocking hook timeout was not enforced: err=%v duration=%s", err, time.Since(started))
	}
}

func TestHookedStoreObserveHookIsAsyncAndIsolated(t *testing.T) {
	base, _ := NewFileStore(nil, nil)
	done := make(chan struct{})
	errorDone := make(chan struct{})
	var mu sync.Mutex
	var hookErrors []error
	dispatcher := NewDispatcher(HookOptions{Timeout: time.Second, ErrorHandler: func(err error) {
		mu.Lock()
		hookErrors = append(hookErrors, err)
		mu.Unlock()
		close(errorDone)
	}})
	dispatcher.AddObserve(func(ctx context.Context, _ Event) error {
		select {
		case <-time.After(40 * time.Millisecond):
		case <-ctx.Done():
		}
		close(done)
		return errors.New("observe failed")
	})
	started := time.Now()
	store := NewHookedStore(base, dispatcher)
	if err := store.Append(context.Background(), Event{Type: "observed"}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed >= 35*time.Millisecond {
		t.Fatalf("append waited for observe hook: %s", elapsed)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("observe hook did not run")
	}
	select {
	case <-errorDone:
	case <-time.After(time.Second):
		t.Fatal("observe hook error was not reported")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hookErrors) != 1 || !strings.Contains(hookErrors[0].Error(), "observe failed") {
		t.Fatalf("unexpected observe hook errors: %v", hookErrors)
	}
}

func TestHookedStoreObservePanicDoesNotAffectFacts(t *testing.T) {
	base, _ := NewFileStore(nil, nil)
	errorSeen := make(chan error, 1)
	dispatcher := NewDispatcher(HookOptions{ErrorHandler: func(err error) { errorSeen <- err }})
	dispatcher.AddObserve(func(context.Context, Event) error { panic("boom") })
	store := NewHookedStore(base, dispatcher)
	if err := store.Append(context.Background(), Event{Type: "fact"}); err != nil {
		t.Fatal(err)
	}
	events, err := base.List(context.Background(), 10)
	if err != nil || len(events) != 1 || events[0].Type != "fact" {
		t.Fatalf("fact event missing after observe panic: events=%#v err=%v", events, err)
	}
	select {
	case err := <-errorSeen:
		if !strings.Contains(err.Error(), "panic") {
			t.Fatalf("unexpected panic error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("observe panic was not reported")
	}
}
