package state

import (
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestGetTimingSnapshotIgnoresNewEmptySessionDefaults(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	sm.EnsureSession(sessionID, 42, 0, 1)

	snapshot := sm.GetTimingSnapshot(sessionID)
	if !snapshot.LastUserMessageAt.IsZero() || !snapshot.LastAssistantMessageAt.IsZero() || !snapshot.LastInteractionAt.IsZero() {
		t.Fatalf("expected empty timing snapshot for new empty session, got %#v", snapshot)
	}
}

func TestGetTimingSnapshotReturnsRecordedTiming(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	userAt := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	assistantAt := userAt.Add(2 * time.Minute)

	sm.EnsureSession(sessionID, 42, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "hello"}, userAt)
	sm.RecordAssistantTurn(sessionID, "hi", assistantAt, false)

	snapshot := sm.GetTimingSnapshot(sessionID)
	if !snapshot.LastUserMessageAt.Equal(userAt) {
		t.Fatalf("expected last user message at %s, got %s", userAt, snapshot.LastUserMessageAt)
	}
	if !snapshot.LastAssistantMessageAt.Equal(assistantAt) {
		t.Fatalf("expected last assistant message at %s, got %s", assistantAt, snapshot.LastAssistantMessageAt)
	}
	if !snapshot.LastInteractionAt.Equal(assistantAt) {
		t.Fatalf("expected last interaction at %s, got %s", assistantAt, snapshot.LastInteractionAt)
	}
}
