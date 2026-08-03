package inbound

import (
	"context"
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/model"
	"project-yume/internal/state"
)

func TestAggregatorMarksPendingUserTurnForTargetMessage(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	cfg := config.GetConfig()
	oldTargetID := cfg.TargetId
	t.Cleanup(func() {
		cfg.TargetId = oldTargetID
	})
	cfg.TargetId = 42

	msgTime := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	msg := model.Msg{
		Message:   "hello",
		User_id:   42,
		MessageID: 1001,
		Time:      msgTime.Unix(),
		Type:      1,
	}

	aggregator := NewMessageAggregator()
	out := make(chan model.Msg, 1)
	aggregator.handleMessage(context.Background(), msg, out)

	sessionID := state.BuildSessionID(msg.User_id, msg.Group_id, msg.Type)
	if !sm.HasPendingOrRunningUserTurn(sessionID) {
		t.Fatalf("expected aggregator to mark pending user turn")
	}
}

func TestAggregatorDoesNotMarkPendingForNonTargetMessage(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	cfg := config.GetConfig()
	oldTargetID := cfg.TargetId
	t.Cleanup(func() {
		cfg.TargetId = oldTargetID
	})
	cfg.TargetId = 42

	msg := model.Msg{
		Message:   "hello",
		User_id:   99,
		MessageID: 1001,
		Time:      time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC).Unix(),
		Type:      1,
	}

	aggregator := NewMessageAggregator()
	out := make(chan model.Msg, 1)
	aggregator.handleMessage(context.Background(), msg, out)

	sessionID := state.BuildSessionID(msg.User_id, msg.Group_id, msg.Type)
	if sm.HasPendingOrRunningUserTurn(sessionID) {
		t.Fatalf("expected non-target message not to mark pending user turn")
	}
}
