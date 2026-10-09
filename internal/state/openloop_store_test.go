package state

import (
	"testing"
	"time"

	"project-yume/internal/domain/intent"
	"project-yume/internal/domain/openloop"
)

func TestCloseOpenLoopCancelsLinkedIntentAndRecordsReason(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	loop := openloop.NewExplicit(42, PrivateSessionID(42), string(openloop.KindFollowUp), "待跟进事项", "turn-1", now)
	if _, err := sm.OpenLoopStore().Upsert(loop); err != nil {
		t.Fatal(err)
	}
	item := intent.NewProactiveContact(42, PrivateSessionID(42), "跟进提醒", now.Add(time.Hour))
	item.OpenLoopID = loop.ID
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	closed, err := sm.CloseOpenLoop(loop.ID, "用户已确认解决", "user", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != openloop.StatusResolved || closed.ClosedReason != "用户已确认解决" || closed.ClosedBy != "user" {
		t.Fatalf("unexpected closed loop: %+v", closed)
	}
	updated, _ := sm.GetIntent(item.ID)
	if updated.Status != intent.StatusCancelled {
		t.Fatalf("linked intent was not cancelled: %+v", updated)
	}
}

func TestDeferOpenLoopSynchronizesLinkedIntent(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	loop := openloop.NewExplicit(42, PrivateSessionID(42), string(openloop.KindFollowUp), "待跟进事项", "turn-1", now)
	if _, err := sm.OpenLoopStore().Upsert(loop); err != nil {
		t.Fatal(err)
	}
	item := intent.NewProactiveContact(42, PrivateSessionID(42), "跟进提醒", now.Add(time.Hour))
	item.OpenLoopID = loop.ID
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	due := now.Add(2 * time.Hour)
	if _, err := sm.DeferOpenLoop(loop.ID, due, "user", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	updated, _ := sm.GetIntent(item.ID)
	if updated.Status != intent.StatusDeferred || !updated.DueAt.Equal(due) {
		t.Fatalf("linked intent was not deferred with loop: %+v", updated)
	}
}
