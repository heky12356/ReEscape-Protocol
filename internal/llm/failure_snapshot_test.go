package llm

import (
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestFailureSnapshotRedactsRequestContent(t *testing.T) {
	request := openai.ChatCompletionRequest{
		Model: "test-model",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "private message"},
			{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
				ID: "call-1", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "search", Arguments: `{"query":"private"}`},
			}}},
		},
		Tools: []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "search", Description: "private tool description"}}},
	}
	snapshot := NewFailureSnapshot(TaskReply, "fake", "test-model", "req-1", request, &testSnapshotError{message: "authorization=secret-value"})
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(encoded)
	for _, secret := range []string{"private message", "private", "secret-value", "private tool description"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("snapshot contains unsanitized value %q: %s", secret, serialized)
		}
	}
	if snapshot.Request.Messages[0].ContentHash == "" || snapshot.Request.Messages[1].ToolCalls[0].ArgumentsHash == "" {
		t.Fatalf("expected content hashes: %#v", snapshot)
	}
	if snapshot.Error != "authorization=[REDACTED]" {
		t.Fatalf("unexpected sanitized error: %q", snapshot.Error)
	}
}

func TestFailureSnapshotReplayIsPlaceholderOnly(t *testing.T) {
	request := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "private message"}},
	}
	snapshot := NewFailureSnapshot(TaskReply, "fake", "test-model", "req-1", request, nil)
	replay, err := snapshot.ReplayRequest()
	if err != nil {
		t.Fatal(err)
	}
	if replay.Model != "test-model" || len(replay.Messages) != 1 || replay.Messages[0].Content != "[REDACTED CONTENT]" {
		t.Fatalf("unexpected replay request: %#v", replay)
	}
	if replay.Stream || len(replay.Tools) != 0 {
		t.Fatalf("replay request should be inert and tool-free: %#v", replay)
	}
}

type testSnapshotError struct{ message string }

func (e *testSnapshotError) Error() string { return e.message }
