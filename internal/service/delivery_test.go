package service

import (
	"context"
	"testing"
	"time"

	"project-yume/internal/domain/intent"
	"project-yume/internal/state"
)

func TestDeliverReplyEmpty(t *testing.T) {
	result := DeliverReply(context.Background(), nil, DeliveryRequest{Reply: "  "})
	if result.Status != DeliveryResultEmpty || len(result.Items) != 0 {
		t.Fatalf("unexpected empty delivery result: %#v", result)
	}
}

func TestApplyDeliveryToIntentCompletesOnlyFullDelivery(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	item := intent.NewProactiveContact(42, state.PrivateSessionID(42), "提醒", now.Add(-time.Minute))
	item.Status = intent.StatusClaimed
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	if err := ApplyDeliveryToIntent(DeliveryResult{IntentID: item.ID, Status: DeliveryResultPartial}, now); err != nil {
		t.Fatal(err)
	}
	updated, _ := sm.GetIntent(item.ID)
	if updated.Status != intent.StatusDeferred {
		t.Fatalf("partial delivery status = %s, want deferred", updated.Status)
	}
	if updated.RetryCount != 1 || updated.DueAt.Before(now) {
		t.Fatalf("partial delivery retry metadata = %+v", updated)
	}
	if err := ApplyDeliveryToIntent(DeliveryResult{IntentID: item.ID, Status: DeliveryResultDelivered}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	updated, _ = sm.GetIntent(item.ID)
	if updated.Status != intent.StatusCompleted {
		t.Fatalf("full delivery status = %s, want completed", updated.Status)
	}
}

func TestApplyDeliveryToIntentPausesAfterRetryLimit(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	item := intent.NewProactiveContact(42, state.PrivateSessionID(42), "提醒", now.Add(-time.Minute))
	item.Status = intent.StatusClaimed
	item.MaxRetries = 1
	if _, err := sm.IntentStore().Upsert(item); err != nil {
		t.Fatal(err)
	}
	if err := ApplyDeliveryToIntent(DeliveryResult{IntentID: item.ID, Status: DeliveryResultFailed, Error: "offline"}, now); err != nil {
		t.Fatal(err)
	}
	updated, _ := sm.GetIntent(item.ID)
	if updated.Status != intent.StatusPaused || updated.RetryCount != 1 || updated.LastDeliveryError != "offline" {
		t.Fatalf("retry exhaustion metadata = %+v", updated)
	}
}

func TestDeliverReplyCancelledBeforeFirstItem(t *testing.T) {
	cancel := make(chan struct{})
	close(cancel)
	result := DeliverReply(context.Background(), nil, DeliveryRequest{Reply: "第一句$第二句", Cancel: cancel})
	if result.Status != DeliveryResultCancelled || result.FirstCommitted {
		t.Fatalf("unexpected cancelled delivery result: %#v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Status != DeliveryCancelled || result.Items[1].Status != DeliveryCancelled {
		t.Fatalf("expected all items cancelled: %#v", result.Items)
	}
}

func TestDeliverReplyPreservesIntentAssociations(t *testing.T) {
	result := DeliverReply(context.Background(), nil, DeliveryRequest{
		TurnID: "turn-1", SourceTurnID: "source-1", SessionID: "private:42", UserID: 42,
		IntentID: "intent-1", OpenLoopID: "loop-1", Reply: " ",
	})
	if result.Status != DeliveryResultEmpty {
		t.Fatalf("unexpected status: %#v", result)
	}
	if result.IntentID != "intent-1" || result.OpenLoopID != "loop-1" || result.SourceTurnID != "source-1" {
		t.Fatalf("delivery associations were lost: %#v", result)
	}
}
