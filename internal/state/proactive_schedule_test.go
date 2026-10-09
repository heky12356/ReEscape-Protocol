package state

import (
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestStateManagerSetGetProactiveSchedule(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	userAt := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	proactiveAt := userAt.Add(30 * time.Minute)
	nextAt := userAt.Add(2 * time.Hour)
	updatedAt := userAt.Add(time.Minute)

	sm.EnsureSession(sessionID, 42, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "hello"}, userAt)
	sm.RecordAssistantTurn(sessionID, "hi", proactiveAt, true)
	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		NextScheduledAt: nextAt,
		Summary:         "晚上继续聊",
		Reason:          "用户明确约定",
		Meta: map[string]any{
			"topic": "schedule",
			"tags":  []string{"react", "proactive"},
		},
		Manual:    true,
		UpdatedAt: updatedAt,
		UpdatedBy: "user",
	})

	schedule := sm.GetProactiveSchedule(sessionID)
	if !schedule.NextScheduledAt.Equal(nextAt) || !schedule.LastInteractionAt.Equal(proactiveAt) || !schedule.LastProactiveAt.Equal(proactiveAt) {
		t.Fatalf("unexpected schedule timing: %#v", schedule)
	}
	if !schedule.Manual || schedule.Summary != "晚上继续聊" || schedule.Reason != "用户明确约定" || schedule.UpdatedBy != "user" {
		t.Fatalf("unexpected schedule metadata: %#v", schedule)
	}
	tags, ok := schedule.Meta["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "react" {
		t.Fatalf("expected cloned tags metadata, got %#v", schedule.Meta)
	}

	tags[0] = "changed"
	again := sm.GetProactiveSchedule(sessionID)
	if again.Meta["tags"].([]string)[0] != "react" {
		t.Fatalf("expected metadata to be cloned on read, got %#v", again.Meta)
	}
}

func TestStateManagerAutomaticScheduleClearsManualMetadata(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	manualAt := time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC)
	autoAt := manualAt.Add(time.Hour)

	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		NextScheduledAt: manualAt,
		Summary:         "手动计划",
		Reason:          "用户约定",
		Meta:            map[string]any{"topic": "manual"},
		Manual:          true,
		UpdatedBy:       "user",
	})
	sm.SetNextScheduledAt(sessionID, autoAt)

	schedule := sm.GetProactiveSchedule(sessionID)
	if !schedule.NextScheduledAt.Equal(autoAt) {
		t.Fatalf("expected automatic next schedule %s, got %s", autoAt, schedule.NextScheduledAt)
	}
	if schedule.Manual || schedule.Summary != "" || schedule.Reason != "automatic_reschedule" || len(schedule.Meta) != 0 {
		t.Fatalf("expected automatic schedule to clear manual metadata, got %#v", schedule)
	}
}

func TestStateManagerClearProactiveScheduleMarksManualCancel(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		NextScheduledAt: time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC),
		Summary:         "手动计划",
		Reason:          "用户约定",
		Manual:          true,
	})
	sm.ClearProactiveSchedule(sessionID, "用户说今天别找了", "user")

	schedule := sm.GetProactiveSchedule(sessionID)
	if !schedule.NextScheduledAt.IsZero() || !schedule.Manual || schedule.Reason != "用户说今天别找了" || schedule.UpdatedBy != "user" {
		t.Fatalf("unexpected cleared schedule: %#v", schedule)
	}
}

func TestStateManagerResetClearsProactiveSchedule(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		NextScheduledAt: time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC),
		Summary:         "手动计划",
		Reason:          "用户约定",
		Meta:            map[string]any{"topic": "manual"},
		Manual:          true,
	})
	sm.ResetSession(sessionID)

	schedule := sm.GetProactiveSchedule(sessionID)
	if !schedule.NextScheduledAt.IsZero() || schedule.Manual || schedule.Summary != "" || schedule.Reason != "" || len(schedule.Meta) != 0 {
		t.Fatalf("expected reset to clear schedule, got %#v", schedule)
	}
}

func TestStateManagerSnapshotCopiesProactiveSchedule(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		NextScheduledAt: time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC),
		Summary:         "手动计划",
		Reason:          "用户约定",
		Meta:            map[string]any{"tags": []string{"a", "b"}},
		Manual:          true,
		UpdatedBy:       "user",
	})

	sm.mu.RLock()
	snapshot := sm.snapshotLocked()
	sm.mu.RUnlock()

	copied := snapshot[sessionID]
	if copied == nil || !copied.ProactiveScheduleManual || copied.ProactiveScheduleSummary != "手动计划" {
		t.Fatalf("expected snapshot schedule fields, got %#v", copied)
	}
	copied.ProactiveScheduleMeta["tags"].([]string)[0] = "changed"
	original := sm.GetProactiveSchedule(sessionID)
	if original.Meta["tags"].([]string)[0] != "a" {
		t.Fatalf("expected snapshot metadata to be cloned, got %#v", original.Meta)
	}
}

func TestLoadSessionsWithoutProactiveScheduleFields(t *testing.T) {
	sm := &StateManager{}
	data := []byte(`{"sessions":{"private:old":{"id":"private:old","user_id":7,"chat_type":1,"conversation":[],"last_updated":"2026-08-01T00:00:00Z"}}}`)

	if err := sm.loadSessionsFromBytes(data); err != nil {
		t.Fatalf("load sessions failed: %v", err)
	}
	schedule := sm.GetProactiveSchedule("private:old")
	if schedule.SessionID != "private:old" || schedule.Manual || !schedule.NextScheduledAt.IsZero() {
		t.Fatalf("unexpected schedule for legacy session: %#v", schedule)
	}
}

func TestLoadSessionsMigratesManualScheduleToLegacyIntent(t *testing.T) {
	sm := &StateManager{}
	data := []byte(`{"sessions":{"private:old":{"id":"private:old","user_id":7,"chat_type":1,"conversation":[],"next_scheduled_at":"2026-08-01T01:00:00Z","proactive_schedule_summary":"继续跟进项目","proactive_schedule_manual":true,"proactive_schedule_updated_at":"2026-08-01T00:00:00Z"}}}`)

	if err := sm.loadSessionsFromBytes(data); err != nil {
		t.Fatalf("load sessions failed: %v", err)
	}
	schedule := sm.GetProactiveSchedule("private:old")
	if schedule.IntentID != "intent:legacy-schedule:private:old" {
		t.Fatalf("unexpected migrated intent id: %#v", schedule)
	}
	item, ok := sm.GetIntent(schedule.IntentID)
	if !ok || item.Source != "legacy_schedule" || item.Summary != "继续跟进项目" || !item.DueAt.Equal(schedule.NextScheduledAt) {
		t.Fatalf("unexpected migrated intent: %#v %v", item, ok)
	}
}

func TestAutomaticScheduleClearsLegacyIntentProjection(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	sessionID := "private:42"
	sm.SetProactiveSchedule(sessionID, ProactiveSchedule{
		IntentID: "intent:legacy-schedule:private:42", NextScheduledAt: time.Now().Add(time.Hour),
		Summary: "手动计划", Manual: true,
	})
	sm.SetNextScheduledAt(sessionID, time.Now().Add(2*time.Hour))
	if schedule := sm.GetProactiveSchedule(sessionID); schedule.IntentID != "" || schedule.Manual {
		t.Fatalf("automatic schedule retained legacy intent projection: %#v", schedule)
	}
}
