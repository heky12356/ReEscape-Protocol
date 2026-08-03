package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"project-yume/internal/state"
)

func TestGetProactiveScheduleReturnsEmptyTimesForEmptySchedule(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	tool := NewGetProactiveScheduleTool()
	result, err := tool.Execute(context.Background(), scheduleTestTurn(time.Now(), "private:42"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["session_id"] != "private:42" || data["next_scheduled_at"] != "" || data["last_proactive_at"] != "" {
		t.Fatalf("unexpected empty schedule data: %#v", data)
	}
}

func TestGetProactiveScheduleReturnsCurrentSessionOnly(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	nextAt := time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC)
	sm.SetProactiveSchedule("private:42", state.ProactiveSchedule{
		NextScheduledAt: nextAt,
		Summary:         "晚上继续聊",
		Reason:          "用户约定",
		Meta:            map[string]any{"topic": "schedule"},
		Manual:          true,
		UpdatedBy:       "user",
	})
	sm.SetProactiveSchedule("private:99", state.ProactiveSchedule{
		NextScheduledAt: nextAt.Add(time.Hour),
		Summary:         "other session",
		Reason:          "other",
		Manual:          true,
	})

	tool := NewGetProactiveScheduleTool()
	result, err := tool.Execute(context.Background(), scheduleTestTurn(time.Now(), "private:42"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["next_scheduled_at"] != nextAt.Format(time.RFC3339) || data["manual"] != true {
		t.Fatalf("unexpected schedule data: %#v", data)
	}
	if data["summary"] == "other session" {
		t.Fatalf("leaked schedule from another session: %#v", data)
	}
	meta, ok := data["meta"].(map[string]any)
	if !ok || meta["topic"] != "schedule" {
		t.Fatalf("unexpected metadata: %#v", data["meta"])
	}
}
