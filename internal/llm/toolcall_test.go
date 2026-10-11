package llm

import (
	"errors"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func assistantToolMessage(ids ...string) openai.ChatCompletionMessage {
	calls := make([]openai.ToolCall, 0, len(ids))
	for _, id := range ids {
		calls = append(calls, openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool"}})
	}
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: calls}
}

func toolResultMessage(id string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: id, Content: "ok"}
}

func TestValidateToolCallHistoryAcceptsOrderedResults(t *testing.T) {
	history := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		assistantToolMessage("call-a", "call-b"),
		toolResultMessage("call-a"),
		toolResultMessage("call-b"),
		{Role: openai.ChatMessageRoleAssistant, Content: "done"},
	}
	if err := ValidateToolCallHistory(history); err != nil {
		t.Fatalf("expected valid tool call history, got %v", err)
	}
}

func TestValidateToolCallHistoryRejectsMalformedSequences(t *testing.T) {
	tests := []struct {
		name    string
		history []openai.ChatCompletionMessage
		code    string
	}{
		{
			name:    "orphan result",
			history: []openai.ChatCompletionMessage{toolResultMessage("call-a")},
			code:    "orphan tool result",
		},
		{
			name:    "unknown call id",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a"), toolResultMessage("call-b")},
			code:    "unknown tool_call_id",
		},
		{
			name:    "out of order",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a", "call-b"), toolResultMessage("call-b"), toolResultMessage("call-a")},
			code:    "tool result is out of order",
		},
		{
			name:    "duplicate result",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a", "call-b"), toolResultMessage("call-a"), toolResultMessage("call-a")},
			code:    "duplicate tool result",
		},
		{
			name:    "missing result",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a")},
			code:    "tool call is missing a result",
		},
		{
			name:    "interleaved message",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a"), {Role: openai.ChatMessageRoleUser, Content: "interrupted"}},
			code:    "tool result must follow its assistant tool call",
		},
		{
			name:    "duplicate call id",
			history: []openai.ChatCompletionMessage{assistantToolMessage("call-a", "call-a")},
			code:    "duplicate tool_call_id",
		},
		{
			name:    "missing call id",
			history: []openai.ChatCompletionMessage{assistantToolMessage("")},
			code:    "assistant tool call has no call_id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateToolCallHistory(test.history)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			var validationErr *ToolCallValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected ToolCallValidationError, got %T: %v", err, err)
			}
			if validationErr.Code != test.code {
				t.Fatalf("error code = %q, want %q", validationErr.Code, test.code)
			}
		})
	}
}

func TestValidateToolCallResponseChecksHistoryAndCallIDs(t *testing.T) {
	history := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hello"}}
	if err := ValidateToolCallResponse(history, assistantToolMessage("call-a")); err != nil {
		t.Fatalf("expected valid response, got %v", err)
	}
	if err := ValidateToolCallResponse(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "done"}); err != nil {
		t.Fatalf("expected normal response, got %v", err)
	}

	badRole := assistantToolMessage("call-a")
	badRole.Role = openai.ChatMessageRoleTool
	err := ValidateToolCallResponse(history, badRole)
	if err == nil || !strings.Contains(err.Error(), "assistant message") {
		t.Fatalf("expected assistant role error, got %v", err)
	}

	badHistory := []openai.ChatCompletionMessage{toolResultMessage("call-a")}
	if err := ValidateToolCallResponse(badHistory, assistantToolMessage("call-b")); err == nil {
		t.Fatalf("expected invalid history to be rejected")
	}

	completedHistory := []openai.ChatCompletionMessage{
		assistantToolMessage("call-a"),
		toolResultMessage("call-a"),
	}
	if err := ValidateToolCallResponse(completedHistory, assistantToolMessage("call-a")); err == nil || !strings.Contains(err.Error(), "duplicate tool_call_id") {
		t.Fatalf("expected response call id reuse to be rejected, got %v", err)
	}
}
