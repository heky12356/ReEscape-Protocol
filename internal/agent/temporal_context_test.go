package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/skill"
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

func TestBuildCurrentTurnContextWrapsTemporalContext(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04:05")

	turn := NewTurnContext(TurnInput{
		Message:       "这是真实用户消息，不应复制到 runtime context",
		ReferenceTime: time.Date(2026, 8, 1, 6, 30, 12, 0, time.UTC),
	})

	context := buildCurrentTurnContext(turn)

	assertTemporalContains(t, context, "【Current Turn Context】")
	assertTemporalContains(t, context, "不是用户原文")
	assertTemporalContains(t, context, "本轮触发来源：message")
	assertTemporalContains(t, context, "当前本地时间：2026-08-01 14:30:12")
	if strings.Contains(context, turn.Message()) {
		t.Fatalf("did not expect current turn context to duplicate user message: %q", context)
	}
}

func TestBuildCurrentTurnContextIncludesPendingInterruptedReply(t *testing.T) {
	sessionID := "agent-test-interrupted"
	state.GetManager().SetInterruptedReply(sessionID, state.InterruptedReply{
		DeliveredSegments: []string{"第一句"}, UndeliveredSegments: []string{"第二句"},
		Summary: "未发送内容：第二句", Status: "pending",
	})
	t.Cleanup(func() { state.GetManager().ClearInterruptedReply(sessionID, "") })

	turn := NewTurnContext(TurnInput{SessionID: sessionID, ReferenceTime: time.Now()})
	context := buildCurrentTurnContext(turn)
	if !strings.Contains(context, "【上一轮投递状态】") || !strings.Contains(context, "未发送内容：第二句") {
		t.Fatalf("expected interrupted reply context, got %q", context)
	}
}

func TestBuildCurrentTurnContextOmitsTemporalDataWhenTimeContextDisabled(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04:05")
	restoreSkillConfig(t, false, 2)

	turn := NewTurnContext(TurnInput{ReferenceTime: time.Now()})
	context := buildCurrentTurnContext(turn)
	assertTemporalContains(t, context, "【Current Turn Context】")
	assertTemporalContains(t, context, "本轮触发来源：message")
	if strings.Contains(context, "当前本地时间") {
		t.Fatalf("did not expect temporal data when time context is disabled, got %q", context)
	}
}

func TestBuildCurrentTurnContextIncludesProactiveTriggerTiming(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04:05")

	triggerTime := time.Date(2026, 8, 1, 7, 15, 0, 0, time.UTC)
	turn := NewTurnContext(TurnInput{
		ReferenceTime: triggerTime,
		EndedAt:       triggerTime,
		Trigger:       TriggerProactive,
	})

	context := buildCurrentTurnContext(turn)

	assertTemporalContains(t, context, "【Current Turn Context】")
	assertTemporalContains(t, context, "本轮触发来源：proactive")
	assertTemporalContains(t, context, "当前主动触发时间：2026-08-01 15:15:00")
	if strings.Contains(context, "当前用户消息发送于") {
		t.Fatalf("did not expect proactive current turn context to use user message timing: %q", context)
	}
}

func TestBuildCurrentTurnContextIncludesSkillCandidates(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04:05")
	restoreSkillConfig(t, true, 2)
	loadAgentTestSkill(t, "comfort", "sad overwhelmed emotional support", "# Comfort")

	turn := NewTurnContext(TurnInput{
		SessionID: "private:42",
		Message:   "sad overwhelmed emotional support",
	})

	context := buildCurrentTurnContext(turn)
	assertTemporalContains(t, context, "【Current Turn Context】")
	assertTemporalContains(t, context, "【Skill Candidates】")
	assertTemporalContains(t, context, "- comfort: sad overwhelmed emotional support")
	if strings.Contains(context, "# Comfort") {
		t.Fatalf("did not expect full skill body in current turn context: %q", context)
	}
}

func TestBuildCurrentTurnContextIncludesAutoLoadedSkillBody(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04:05")
	restoreSkillConfig(t, true, 2)
	loadAgentTestSkillWithFrontmatter(
		t,
		"paper-humanizer",
		"Polish academic writing.",
		"triggers:\n  - 润色这段论文\n",
		"# Paper Humanizer\nFollow the full workflow.",
	)

	context := buildCurrentTurnContext(NewTurnContext(TurnInput{
		SessionID: "private:43",
		Message:   "帮我润色这段论文",
	}))
	assertTemporalContains(t, context, "【Activated Skills】")
	assertTemporalContains(t, context, "【Skill: paper-humanizer】")
	assertTemporalContains(t, context, "Follow the full workflow.")
	if strings.Contains(context, "【Skill Candidates】") {
		t.Fatalf("did not expect auto-loaded skill to remain a candidate: %q", context)
	}
}

func TestResolveTurnSkillsEmitsActivationEventsOnlyOnce(t *testing.T) {
	restoreSkillConfig(t, true, 2)
	loadAgentTestSkillWithFrontmatter(
		t,
		"paper-humanizer",
		"Polish academic writing.",
		"triggers:\n  - 润色这段论文\n",
		"# Paper Humanizer",
	)
	turn := NewTurnContext(TurnInput{
		SessionID: "private:44",
		UserID:    44,
		Message:   "帮我润色这段论文",
	})

	events := resolveTurnSkills(turn)
	eventTypes := make(map[string]bool)
	for _, event := range events {
		eventTypes[event.Type] = true
	}
	if !eventTypes["skill_candidates_selected"] ||
		!eventTypes["skill_activated"] ||
		!eventTypes["skill_auto_loaded"] {
		t.Fatalf("expected activation audit events, got %#v", events)
	}
	if repeated := resolveTurnSkills(turn); len(repeated) != 0 {
		t.Fatalf("expected skill resolution to be cached for the turn, got %#v", repeated)
	}
}

func TestBuildCurrentTurnContextOmitsSkillsWhenDisabled(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04:05")
	restoreSkillConfig(t, false, 2)
	loadAgentTestSkill(t, "comfort", "Use when the user feels sad or overwhelmed.", "# Comfort")

	context := buildCurrentTurnContext(NewTurnContext(TurnInput{
		SessionID: "private:42",
		Message:   "I feel sad",
	}))
	if strings.Contains(context, "【Skill Candidates】") || strings.Contains(context, "【Activated Skills】") {
		t.Fatalf("did not expect skills when disabled: %q", context)
	}
}

func TestRuntimeBuildMessagesPlacesTemporalContextInCurrentTurnPacket(t *testing.T) {
	restoreTemporalConfig(t, true, "Asia/Shanghai", "2006-01-02 15:04")
	restoreSkillConfig(t, false, 2)
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
		Message:       "昨天那个事情我想了一下",
		ReferenceTime: messageTime,
		StartedAt:     messageTime,
		EndedAt:       messageTime,
	}))

	if len(messages) != 2 {
		t.Fatalf("expected system prompt and current turn packet, got %#v", messages)
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[0].Content, "【Agent Policy】") {
		t.Fatalf("expected first message to be agent system prompt, got %#v", messages[0])
	}
	last := messages[len(messages)-1]
	if last.Role != openai.ChatMessageRoleUser ||
		!strings.Contains(last.Content, "【Current Turn Context】") ||
		!strings.Contains(last.Content, "当前用户消息发送于：2026-08-02 09:30") ||
		!strings.Contains(last.Content, "不是用户原文") ||
		!strings.Contains(last.Content, "【User Message】") {
		t.Fatalf("expected final message to be current turn packet, got %#v", last)
	}
	if strings.Contains(messages[0].Content, "2026-08-02 09:30") {
		t.Fatalf("did not expect concrete current turn time in system prompt: %q", messages[0].Content)
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

func restoreSkillConfig(t *testing.T, enabled bool, limit int) {
	t.Helper()

	cfg := config.GetConfig()
	previousEnabled := cfg.EnableSkills
	previousLimit := cfg.SkillAutoHintLimit
	previousCandidateMinScore := cfg.SkillCandidateMinScore
	previousAutoLoadMinScore := cfg.SkillAutoLoadMinScore
	previousAutoLoadMinConfidence := cfg.SkillAutoLoadMinConfidence
	previousMaxAutoLoaded := cfg.SkillMaxAutoLoaded
	t.Cleanup(func() {
		cfg.EnableSkills = previousEnabled
		cfg.SkillAutoHintLimit = previousLimit
		cfg.SkillCandidateMinScore = previousCandidateMinScore
		cfg.SkillAutoLoadMinScore = previousAutoLoadMinScore
		cfg.SkillAutoLoadMinConfidence = previousAutoLoadMinConfidence
		cfg.SkillMaxAutoLoaded = previousMaxAutoLoaded
		skill.GetManager().LoadDirs(nil)
	})

	cfg.EnableSkills = enabled
	cfg.SkillAutoHintLimit = limit
	cfg.SkillCandidateMinScore = 8
	cfg.SkillAutoLoadMinScore = 16
	cfg.SkillAutoLoadMinConfidence = 0.75
	cfg.SkillMaxAutoLoaded = 1
}

func loadAgentTestSkill(t *testing.T, name, description, body string) {
	loadAgentTestSkillWithFrontmatter(t, name, description, "", body)
}

func loadAgentTestSkillWithFrontmatter(t *testing.T, name, description, frontmatter, body string) {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n" + frontmatter + "---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := skill.GetManager().LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skill: %v", errs)
	}
}

func assertTemporalContains(t *testing.T, content string, expected string) {
	t.Helper()
	if !strings.Contains(content, expected) {
		t.Fatalf("expected %q to contain %q", content, expected)
	}
}
