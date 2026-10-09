package state

import (
	"encoding/json"
	"testing"
	"time"

	"project-yume/internal/domain/intent"
	"project-yume/internal/domain/openloop"
)

func TestIntentStoreClaimsDueIntentOnlyOnce(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	item := intent.NewProactiveContact(42, PrivateSessionID(42), "测试提醒", now.Add(-time.Minute))
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	claimed, ok := sm.IntentStore().ClaimDue(42, PrivateSessionID(42), now)
	if !ok || claimed.Status != intent.StatusClaimed {
		t.Fatalf("unexpected claim: %+v %v", claimed, ok)
	}
	if _, ok := sm.IntentStore().ClaimDue(42, PrivateSessionID(42), now); ok {
		t.Fatal("claimed intent must not be claimed twice")
	}
	completed, err := sm.IntentStore().Transition(claimed.ID, intent.StatusCompleted, now)
	if err != nil || completed.Status != intent.StatusCompleted {
		t.Fatalf("complete failed: %+v %v", completed, err)
	}
}

func TestIntentStoreMaintainsOpenLoopAssociation(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	loop := openloop.NewExplicit(42, PrivateSessionID(42), string(openloop.KindFollowUp), "待跟进事项", "turn-1", now)
	if _, err := sm.OpenLoopStore().Upsert(loop); err != nil {
		t.Fatal(err)
	}
	item := intent.NewProactiveContact(42, PrivateSessionID(42), "提醒跟进", now.Add(time.Hour))
	item.OpenLoopID = loop.ID
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	updated, ok := sm.GetOpenLoop(loop.ID)
	if !ok || updated.IntentID != item.ID {
		t.Fatalf("open loop association missing: %#v %v", updated, ok)
	}

	item.OpenLoopID = ""
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	updated, _ = sm.GetOpenLoop(loop.ID)
	if updated.IntentID != "" {
		t.Fatalf("stale open loop association retained: %#v", updated)
	}
}

func TestIntentStorePersistsInSessionSnapshot(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	item := intent.NewProactiveContact(7, PrivateSessionID(7), "持久化提醒", time.Now().Add(time.Hour))
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(SessionStorage{Sessions: map[string]*Session{}, Intents: map[string]intent.Intent{item.ID: item}}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var decoded SessionStorage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded.Intents[item.ID]; !ok {
		t.Fatalf("intent missing from snapshot: %s", string(data))
	}
}

func TestLoadSessionsReleasesClaimedIntentAfterRestart(t *testing.T) {
	sm := &StateManager{}
	data := []byte(`{"sessions":{"private:42":{"id":"private:42","user_id":42,"chat_type":1,"conversation":[]}},"intents":{"intent:claimed":{"id":"intent:claimed","user_id":42,"session_id":"private:42","kind":"follow_up","summary":"重启后继续提醒","due_at":"2026-08-01T01:00:00Z","status":"claimed","updated_at":"2026-08-01T00:00:00Z"}}}`)
	if err := sm.loadSessionsFromBytes(data); err != nil {
		t.Fatalf("load sessions failed: %v", err)
	}
	item, ok := sm.GetIntent("intent:claimed")
	if !ok || item.Status != intent.StatusPending {
		t.Fatalf("claimed intent was not released on restart: %#v %v", item, ok)
	}
	claimed, ok := sm.IntentStore().ClaimDue(42, PrivateSessionID(42), time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC))
	if !ok || claimed.ID != "intent:claimed" || claimed.Status != intent.StatusClaimed {
		t.Fatalf("recovered intent cannot be claimed: %#v %v", claimed, ok)
	}
}
