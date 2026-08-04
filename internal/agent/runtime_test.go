package agent

import (
	"strings"
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/skill"
	"project-yume/internal/state"
	"project-yume/internal/tools"
	"project-yume/internal/tools/catalog"
	toolskill "project-yume/internal/tools/skill"

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
	last := messages[len(messages)-1]
	if len(messages) < 2 || last.Role != openai.ChatMessageRoleUser || !strings.Contains(last.Content, "【Current Turn Context】") {
		t.Fatalf("expected current turn context at message end, got %#v", messages)
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

func TestRuntimeBuildMessagesKeepsCurrentUserMessageWhenTimeContextDisabled(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
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
		Message:       "关闭时间上下文时直接接 conversation",
		ReferenceTime: messageTime,
		StartedAt:     messageTime,
		EndedAt:       messageTime,
	}))

	if len(messages) != 2 {
		t.Fatalf("expected system prompt and current turn packet only, got %#v", messages)
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[0].Content, "【Agent Policy】") {
		t.Fatalf("expected first message to be system prompt, got %#v", messages[0])
	}
	if messages[1].Role != openai.ChatMessageRoleUser ||
		!strings.Contains(messages[1].Content, "【Current Turn Context】") ||
		!strings.Contains(messages[1].Content, "本轮触发来源：message") ||
		!strings.Contains(messages[1].Content, "【User Message】\n关闭时间上下文时直接接 conversation") {
		t.Fatalf("expected current user message wrapped with current turn context, got %#v", messages[1])
	}
}

func TestRuntimeBuildMessagesPlacesCurrentTurnContextAtEnd(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:102"
	messageTime := time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC)
	sm.EnsureSession(sessionID, 102, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "上一轮问题"}, messageTime.Add(-time.Hour))
	sm.RecordAssistantTurn(sessionID, "上一轮回复", messageTime.Add(-50*time.Minute), false)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "当前问题"}, messageTime)

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:                  sessionID,
		UserID:                     102,
		ChatType:                   1,
		Message:                    "当前问题",
		ReferenceTime:              messageTime,
		StartedAt:                  messageTime,
		EndedAt:                    messageTime,
		PreviousUserMessageAt:      messageTime.Add(-time.Hour),
		PreviousAssistantMessageAt: messageTime.Add(-50 * time.Minute),
		PreviousInteractionAt:      messageTime.Add(-50 * time.Minute),
	}))

	if len(messages) != 4 {
		t.Fatalf("expected system, history user, history assistant, current turn packet, got %#v", messages)
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[0].Content, "【Agent Policy】") {
		t.Fatalf("expected first message to be stable system prompt, got %#v", messages[0])
	}
	if messages[1].Content != "上一轮问题" || messages[2].Content != "上一轮回复" {
		t.Fatalf("expected prior conversation before current turn packet, got %#v", messages)
	}
	last := messages[len(messages)-1]
	if last.Role != openai.ChatMessageRoleUser ||
		!strings.Contains(last.Content, "【Current Turn Context】") ||
		!strings.Contains(last.Content, "当前本地时间：2026-08-02 09:30") ||
		!strings.Contains(last.Content, "【User Message】\n当前问题") {
		t.Fatalf("expected current turn context and user message in final message, got %#v", last)
	}
}

func TestRuntimeBuildMessagesDoesNotDuplicateCurrentUserMessage(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:103"
	messageTime := time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC)
	sm.EnsureSession(sessionID, 103, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "重复内容"}, messageTime.Add(-time.Hour))
	sm.RecordAssistantTurn(sessionID, "我听到了", messageTime.Add(-50*time.Minute), false)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "重复内容"}, messageTime)

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     sessionID,
		UserID:        103,
		ChatType:      1,
		Message:       "重复内容",
		ReferenceTime: messageTime,
		StartedAt:     messageTime,
		EndedAt:       messageTime,
	}))

	if len(messages) != 4 {
		t.Fatalf("expected only the previous repeated message plus current packet, got %#v", messages)
	}
	if messages[1].Role != openai.ChatMessageRoleUser || messages[1].Content != "重复内容" {
		t.Fatalf("expected previous same-content user turn to remain, got %#v", messages[1])
	}
	last := messages[len(messages)-1]
	if last.Content == "重复内容" || !strings.Contains(last.Content, "【User Message】\n重复内容") {
		t.Fatalf("expected current message only inside current turn packet, got %#v", last)
	}
}

func TestRuntimeBuildMessagesKeepsSystemPromptStable(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	first := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     "private:104",
		UserID:        104,
		ChatType:      1,
		Message:       "第一条",
		ReferenceTime: time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC),
	}))
	second := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     "private:104",
		UserID:        104,
		ChatType:      1,
		Message:       "第二条",
		ReferenceTime: time.Date(2026, 8, 3, 1, 30, 12, 0, time.UTC),
	}))

	if first[0].Content != second[0].Content {
		t.Fatalf("expected system prompt to stay stable across turns")
	}
	if strings.Contains(first[0].Content, "2026-08-02") || strings.Contains(second[0].Content, "2026-08-03") {
		t.Fatalf("did not expect concrete turn times in system prompt")
	}
}

func TestRuntimeBuildMessagesPlacesProactiveTaskAtEnd(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:105"
	triggerTime := time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC)
	sm.EnsureSession(sessionID, 105, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "之前聊过"}, triggerTime.Add(-time.Hour))

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     sessionID,
		UserID:        105,
		ChatType:      1,
		ReferenceTime: triggerTime,
		EndedAt:       triggerTime,
		Trigger:       TriggerProactive,
		Actor:         ActorScheduler,
	}))

	if len(messages) != 3 {
		t.Fatalf("expected system, history, proactive current turn packet, got %#v", messages)
	}
	last := messages[len(messages)-1]
	if last.Role != openai.ChatMessageRoleUser ||
		!strings.Contains(last.Content, "【Current Turn Context】") ||
		!strings.Contains(last.Content, "本轮触发来源：proactive") ||
		!strings.Contains(last.Content, "当前主动触发时间：2026-08-02 09:30") ||
		!strings.Contains(last.Content, "【Proactive Task】") {
		t.Fatalf("expected proactive task at final message, got %#v", last)
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

func TestRuntimeToolChoiceForStepCanForceSingleCandidateRead(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.SkillForceReadOnCandidate
	t.Cleanup(func() { cfg.SkillForceReadOnCandidate = previous })
	cfg.SkillForceReadOnCandidate = true

	registry := tools.NewRegistry()
	registry.MustRegister(toolskill.NewReadSkillTool())
	runtime := NewRuntime(registry, nil, Budget{})
	turn := NewTurnContext(TurnInput{})
	turn.SetSkillResolution(nil, []skill.Match{{Name: "comfort", Score: 10, Confidence: 0.6}})

	choice, ok := runtime.toolChoiceForStep(turn, 0).(openai.ToolChoice)
	if !ok || choice.Function.Name != "read_skill" {
		t.Fatalf("expected forced read_skill choice, got %#v", runtime.toolChoiceForStep(turn, 0))
	}
	if runtime.toolChoiceForStep(turn, 1) != "auto" {
		t.Fatalf("expected later steps to use auto tool choice")
	}
}
