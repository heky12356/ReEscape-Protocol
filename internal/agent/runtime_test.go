package agent

import (
	"strings"
	"testing"
	"time"

	"project-yume/internal/state"
	"project-yume/internal/tools"
	"project-yume/internal/tools/catalog"

	openai "github.com/sashabaranov/go-openai"
)

func TestRuntimeBuildMessagesFiltersWriteToolsFromPromptWhenPolicyDisallowsWrites(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	state.GetManager().ClearAllSessions()
	t.Cleanup(state.GetManager().ClearAllSessions)

	registry := catalog.NewRegistry()
	executor := tools.NewExecutor(registry, tools.Policy{AllowWriteTools: false}, time.Second)
	runtime := NewRuntime(registry, executor, Budget{})

	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID: "private:99",
		UserID:    99,
		ChatType:  1,
	}))
	if len(messages) == 0 {
		t.Fatalf("expected at least one system message")
	}
	if len(messages) < 2 || messages[1].Role != openai.ChatMessageRoleUser || !strings.Contains(messages[1].Content, "【Runtime Context】") {
		t.Fatalf("expected runtime context after system prompt, got %#v", messages)
	}

	prompt := messages[0].Content
	forbidden := []string{"remember_fact", "update_profile", "update_affection"}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("did not expect filtered write tool %q in prompt: %q", item, prompt)
		}
	}
	if !strings.Contains(prompt, "当前写记忆工具不可用，不要尝试写入或声称已经记住。") {
		t.Fatalf("expected memory write unavailable guidance, got %q", prompt)
	}
}

func TestRuntimeBuildMessagesOmitsRuntimeContextWhenTimeContextDisabled(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04")
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:101"
	messageTime := time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC)
	sm.EnsureSession(sessionID, 101, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "关闭时间上下文时直接接 conversation",
	}, messageTime)

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     sessionID,
		UserID:        101,
		ChatType:      1,
		ReferenceTime: messageTime,
		StartedAt:     messageTime,
		EndedAt:       messageTime,
	}))

	if len(messages) != 2 {
		t.Fatalf("expected system prompt and conversation only, got %#v", messages)
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[0].Content, "【Agent Policy】") {
		t.Fatalf("expected first message to be system prompt, got %#v", messages[0])
	}
	if messages[1].Role != openai.ChatMessageRoleUser || messages[1].Content != "关闭时间上下文时直接接 conversation" {
		t.Fatalf("expected conversation immediately after system prompt, got %#v", messages[1])
	}
}

func TestIsScheduleManagingTool(t *testing.T) {
	if !isScheduleManagingTool(tools.ExecutionResult{
		ToolName: "update_proactive_schedule",
		Result:   tools.ToolResult{Mutated: true},
	}) {
		t.Fatalf("expected mutated update_proactive_schedule to manage schedule")
	}
	if isScheduleManagingTool(tools.ExecutionResult{
		ToolName: "update_profile",
		Result:   tools.ToolResult{Mutated: true},
	}) {
		t.Fatalf("did not expect ordinary write tool to manage schedule")
	}
	if isScheduleManagingTool(tools.ExecutionResult{
		ToolName: "update_proactive_schedule",
		Result:   tools.ToolResult{Mutated: false},
	}) {
		t.Fatalf("did not expect non-mutating schedule tool result to manage schedule")
	}
}
