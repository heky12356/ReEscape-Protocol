package scheduler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	naturalscheduler "project-yume/internal/scheduler"
	"project-yume/internal/state"
)

type fakeTurnView struct {
	requestID     string
	sessionID     string
	userID        int64
	groupID       int64
	chatType      int
	message       string
	referenceTime time.Time
	trigger       string
	actor         string
}

func (t fakeTurnView) RequestID() string {
	return t.requestID
}

func (t fakeTurnView) SessionID() string {
	return t.sessionID
}

func (t fakeTurnView) UserID() int64 {
	return t.userID
}

func (t fakeTurnView) GroupID() int64 {
	return t.groupID
}

func (t fakeTurnView) ChatType() int {
	return t.chatType
}

func (t fakeTurnView) Message() string {
	return t.message
}

func (t fakeTurnView) ReferenceTime() time.Time {
	return t.referenceTime
}

func (t fakeTurnView) Trigger() string {
	return t.trigger
}

func (t fakeTurnView) Actor() string {
	return t.actor
}

func TestUpdateProactiveScheduleSetWritesManualSchedule(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	reference := time.Date(2026, 8, 1, 14, 0, 0, 0, time.FixedZone("CST", 8*3600))
	scheduled := reference.Add(2 * time.Hour)
	tool := NewUpdateProactiveScheduleTool()

	result, err := tool.Execute(context.Background(), scheduleTestTurn(reference, "private:42"), json.RawMessage(`{
		"action":"set",
		"scheduled_at":"`+scheduled.Format(time.RFC3339)+`",
		"summary":"晚上继续聊 ReAct 调度",
		"reason":"用户明确约定晚上继续",
		"meta":{"topic":"ReAct","source":"user_commitment","tags":["schedule","react"]}
	}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !result.Mutated || len(result.Events) != 1 || result.Events[0].Type != "proactive_schedule_updated" {
		t.Fatalf("expected mutated result with event, got %#v", result)
	}

	schedule := sm.GetProactiveSchedule("private:42")
	if !schedule.Manual || !schedule.NextScheduledAt.Equal(scheduled) || schedule.Summary != "晚上继续聊 ReAct 调度" {
		t.Fatalf("unexpected stored schedule: %#v", schedule)
	}
	if schedule.Reason != "用户明确约定晚上继续" || schedule.UpdatedBy != "user" {
		t.Fatalf("unexpected stored reason/actor: %#v", schedule)
	}
	tags, ok := schedule.Meta["tags"].([]string)
	if !ok || len(tags) != 2 || tags[1] != "react" {
		t.Fatalf("unexpected stored meta: %#v", schedule.Meta)
	}
}

func TestUpdateProactiveScheduleRejectsPastSet(t *testing.T) {
	state.GetManager().ClearAllSessions()
	t.Cleanup(state.GetManager().ClearAllSessions)

	reference := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	tool := NewUpdateProactiveScheduleTool()
	_, err := tool.Execute(context.Background(), scheduleTestTurn(reference, "private:42"), json.RawMessage(`{
		"action":"set",
		"scheduled_at":"2026-08-01T13:59:00Z",
		"reason":"用户约定"
	}`))
	if err == nil || !strings.Contains(err.Error(), "scheduled_at must be at least") {
		t.Fatalf("expected past schedule error, got %v", err)
	}
}

func TestUpdateProactiveScheduleDelayUsesExistingSchedule(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	reference := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	current := reference.Add(30 * time.Minute)
	sm.SetProactiveSchedule("private:42", state.ProactiveSchedule{
		NextScheduledAt: current,
		Summary:         "已有计划",
		Reason:          "用户约定",
		Manual:          true,
	})

	tool := NewUpdateProactiveScheduleTool()
	_, err := tool.Execute(context.Background(), scheduleTestTurn(reference, "private:42"), json.RawMessage(`{
		"action":"delay",
		"delay_minutes":15,
		"summary":"再晚一点",
		"reason":"用户要求推迟"
	}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	schedule := sm.GetProactiveSchedule("private:42")
	if !schedule.NextScheduledAt.Equal(current.Add(15 * time.Minute)) {
		t.Fatalf("expected delay from existing schedule, got %s", schedule.NextScheduledAt)
	}
}

func TestUpdateProactiveScheduleDelayUsesReferenceWhenNoExistingSchedule(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	reference := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	tool := NewUpdateProactiveScheduleTool()
	_, err := tool.Execute(context.Background(), scheduleTestTurn(reference, "private:42"), json.RawMessage(`{
		"action":"delay",
		"delay_minutes":30,
		"reason":"用户说半小时后提醒"
	}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	schedule := sm.GetProactiveSchedule("private:42")
	if !schedule.NextScheduledAt.Equal(reference.Add(30 * time.Minute)) {
		t.Fatalf("expected delay from reference time, got %s", schedule.NextScheduledAt)
	}
}

func TestUpdateProactiveScheduleCancelDoesNotImmediatelyReschedule(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	location := time.FixedZone("CST", 8*3600)
	reference := time.Date(2026, 8, 1, 16, 0, 0, 0, location)
	tool := NewUpdateProactiveScheduleTool()
	_, err := tool.Execute(context.Background(), scheduleTestTurn(reference, "private:42"), json.RawMessage(`{
		"action":"cancel",
		"summary":"用户取消今天主动触达",
		"reason":"用户说今天先别主动找我",
		"meta":{"scope":"today"}
	}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	expected := time.Date(2026, 8, 2, 9, 0, 0, 0, location)
	schedule := sm.GetProactiveSchedule("private:42")
	if !schedule.Manual || !schedule.NextScheduledAt.Equal(expected) {
		t.Fatalf("unexpected cancel schedule: %#v", schedule)
	}

	ns := naturalscheduler.NewNaturalScheduler()
	next := ns.EnsureScheduled("private:42", reference.Add(time.Minute))
	if !next.Equal(expected) {
		t.Fatalf("expected EnsureScheduled to keep cancel boundary, got %s", next)
	}
}

func TestUpdateProactiveScheduleRejectsInvalidJSON(t *testing.T) {
	state.GetManager().ClearAllSessions()
	t.Cleanup(state.GetManager().ClearAllSessions)

	tool := NewUpdateProactiveScheduleTool()
	_, err := tool.Execute(context.Background(), scheduleTestTurn(time.Now(), "private:42"), json.RawMessage(`{"action":`))
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func scheduleTestTurn(reference time.Time, sessionID string) fakeTurnView {
	return fakeTurnView{
		requestID:     "req-1",
		sessionID:     sessionID,
		userID:        42,
		chatType:      1,
		message:       "晚上继续",
		referenceTime: reference,
		trigger:       "message",
		actor:         "user",
	}
}
