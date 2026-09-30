package state

import (
	"encoding/json"
	"testing"
	"time"

	"project-yume/internal/domain/intent"
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
