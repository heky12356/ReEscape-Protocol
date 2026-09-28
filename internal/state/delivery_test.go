package state

import (
	"context"
	"testing"
)

func TestActiveDeliveryCanBeCancelledAndEnded(t *testing.T) {
	sessionID := "delivery-test-active"
	sm := GetManager()
	ctx, cancel, replaced := sm.BeginActiveDelivery(context.Background(), sessionID, "delivery-1", "turn-1")
	if replaced {
		t.Fatal("first delivery should not replace an existing delivery")
	}
	if sm.GetActiveDelivery(sessionID) == nil {
		t.Fatal("expected active delivery")
	}
	if !sm.CancelActiveDelivery(sessionID) {
		t.Fatal("expected active delivery cancellation")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("delivery context was not cancelled")
	}
	cancel()
	sm.EndActiveDelivery(sessionID, "delivery-1")
	if sm.GetActiveDelivery(sessionID) != nil {
		t.Fatal("expected delivery to be ended")
	}
}

func TestInterruptedReplyExpires(t *testing.T) {
	sessionID := "delivery-test-interrupted"
	sm := GetManager()
	sm.SetInterruptedReply(sessionID, InterruptedReply{Status: "pending"})
	got := sm.GetInterruptedReply(sessionID)
	if got == nil || got.Status != "pending" {
		t.Fatalf("unexpected interrupted reply: %#v", got)
	}
	sm.ClearInterruptedReply(sessionID, "discarded")
	got = sm.GetInterruptedReply(sessionID)
	if got == nil || got.Status != "discarded" {
		t.Fatalf("expected discarded interrupted reply: %#v", got)
	}
}
