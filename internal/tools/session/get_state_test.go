package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"project-yume/internal/state"
)

type fakeTurnView struct {
	sessionID     string
	userID        int64
	referenceTime time.Time
}

func (t fakeTurnView) RequestID() string {
	return "req-1"
}

func (t fakeTurnView) SessionID() string {
	return t.sessionID
}

func (t fakeTurnView) UserID() int64 {
	return t.userID
}

func (t fakeTurnView) GroupID() int64 {
	return 0
}

func (t fakeTurnView) ChatType() int {
	return 1
}

func (t fakeTurnView) Message() string {
	return ""
}

func (t fakeTurnView) ReferenceTime() time.Time {
	return t.referenceTime
}

func (t fakeTurnView) Trigger() string {
	return "message"
}

func (t fakeTurnView) Actor() string {
	return "user"
}

func TestGetStateIncludesProactiveScheduleDetails(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	nextAt := time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC)
	sm.SetProactiveSchedule(sessionID, state.ProactiveSchedule{
		NextScheduledAt: nextAt,
		Summary:         "晚上继续聊",
		Reason:          "用户约定",
		Meta:            map[string]any{"topic": "schedule"},
		Manual:          true,
		UpdatedBy:       "user",
	})

	result, err := NewGetStateTool().Execute(context.Background(), fakeTurnView{sessionID: sessionID, userID: 42}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["next_scheduled_at"] != nextAt.Format(time.RFC3339) || data["proactive_schedule_manual"] != true {
		t.Fatalf("unexpected schedule fields: %#v", data)
	}
	if data["proactive_schedule_summary"] != "晚上继续聊" || data["proactive_schedule_reason"] != "用户约定" {
		t.Fatalf("unexpected schedule metadata: %#v", data)
	}
}
