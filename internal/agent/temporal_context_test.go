package agent

import (
	"strings"
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/state"
	"project-yume/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func TestBuildTemporalContextIncludesMessageTimingAndAggregation(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04:05")

	referenceTime := time.Date(2026, 8, 1, 6, 30, 12, 0, time.UTC)
	startedAt := referenceTime.Add(-32 * time.Second)
	endedAt := referenceTime.Add(-14 * time.Second)

	turn := NewTurnContext(TurnInput{
		ReferenceTime:              referenceTime,
		StartedAt:                  startedAt,
		EndedAt:                    endedAt,
		Aggregated:                 true,
		SegmentCount:               3,
		PreviousUserMessageAt:      endedAt.Add(-(23*time.Hour + 12*time.Minute)),
		PreviousAssistantMessageAt: endedAt.Add(-(23*time.Hour + 10*time.Minute)),
		PreviousInteractionAt:      endedAt.Add(-(23*time.Hour + 10*time.Minute)),
	})

	context := buildTemporalContext(turn)

	assertTemporalContains(t, context, "【Temporal Context】")
	assertTemporalContains(t, context, "当前本地时间：2026-08-01 14:30:12")
	assertTemporalContains(t, context, "当前用户消息发送于：2026-08-01 14:29:58")
	assertTemporalContains(t, context, "距离上次用户消息：23 小时 12 分钟。")
	assertTemporalContains(t, context, "距离上次助手回复：23 小时 10 分钟。")
	assertTemporalContains(t, context, "本轮由 3 条连续消息聚合，跨度 18 秒。")
	assertTemporalContains(t, context, "提示：当前对话与上一轮不连续")
}

func TestBuildTemporalContextCanBeDisabled(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04:05")

	turn := NewTurnContext(TurnInput{ReferenceTime: time.Now()})
	if context := buildTemporalContext(turn); context != "" {
		t.Fatalf("expected empty temporal context, got %q", context)
	}
}

func TestRuntimeBuildMessagesInjectsTemporalContextBeforeConversation(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	messageTime := time.Date(2026, 8, 2, 1, 30, 12, 0, time.UTC)
	sm.EnsureSession(sessionID, 42, 0, 1)
	sm.RecordUserTurn(sessionID, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "昨天那个事情我想了一下",
	}, messageTime)

	runtime := NewRuntime(tools.NewRegistry(), nil, Budget{})
	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID:     sessionID,
		UserID:        42,
		ChatType:      1,
		ReferenceTime: messageTime,
		StartedAt:     messageTime,
		EndedAt:       messageTime,
	}))

	if len(messages) < 3 {
		t.Fatalf("expected system prompt, temporal context, and conversation, got %d messages", len(messages))
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[0].Content, "【Agent Policy】") {
		t.Fatalf("expected first message to be agent system prompt, got %#v", messages[0])
	}
	if messages[1].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[1].Content, "【Temporal Context】") {
		t.Fatalf("expected second message to be temporal system context, got %#v", messages[1])
	}
	if messages[2].Role != openai.ChatMessageRoleUser || messages[2].Content != "昨天那个事情我想了一下" {
		t.Fatalf("expected conversation after temporal context, got %#v", messages[2])
	}
}

func restoreTemporalConfig(t *testing.T, enabled bool, timezone string, format string) {
	t.Helper()

	cfg := config.GetConfig()
	previousEnabled := cfg.EnableTimeContext
	previousTimezone := cfg.TimeContextTimezone
	previousFormat := cfg.TimeContextFormat
	t.Cleanup(func() {
		cfg.EnableTimeContext = previousEnabled
		cfg.TimeContextTimezone = previousTimezone
		cfg.TimeContextFormat = previousFormat
	})

	cfg.EnableTimeContext = enabled
	cfg.TimeContextTimezone = timezone
	cfg.TimeContextFormat = format
}

func assertTemporalContains(t *testing.T, content string, expected string) {
	t.Helper()
	if !strings.Contains(content, expected) {
		t.Fatalf("expected %q to contain %q", content, expected)
	}
}
