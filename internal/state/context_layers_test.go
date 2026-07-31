package state

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestExtractConversationTurns(t *testing.T) {
	conversation := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system prompt"},
		{Role: openai.ChatMessageRoleUser, Content: "我今天有点焦虑，想聊聊。"},
		{Role: openai.ChatMessageRoleAssistant, Content: "可以，先告诉我发生了什么。"},
		{Role: openai.ChatMessageRoleUser, Content: "还有个问题没解决？"},
	}

	turns := extractConversationTurns(conversation)
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}

	if turns[0].User != "我今天有点焦虑，想聊聊。" {
		t.Fatalf("unexpected first user turn: %q", turns[0].User)
	}
	if !turns[0].HasReply || turns[0].Assistant != "可以，先告诉我发生了什么。" {
		t.Fatalf("expected first turn to include assistant reply, got %+v", turns[0])
	}

	if turns[1].User != "还有个问题没解决？" {
		t.Fatalf("unexpected second user turn: %q", turns[1].User)
	}
	if turns[1].HasReply {
		t.Fatalf("expected second turn to be unmatched, got %+v", turns[1])
	}
	if !turns[1].IsUnmatched {
		t.Fatalf("expected second turn to be marked unmatched, got %+v", turns[1])
	}
}

func TestBuildConversationContextLayersRollingSummaryUsesOlderTurns(t *testing.T) {
	conversation := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "我是后端工程师，最近在排查登录问题。"},
		{Role: openai.ChatMessageRoleAssistant, Content: "我会先帮你梳理排查路径。"},
		{Role: openai.ChatMessageRoleUser, Content: "这两天有点焦虑，因为线上一直报错。"},
		{Role: openai.ChatMessageRoleAssistant, Content: "先把错误栈和复现条件整理出来。"},
		{Role: openai.ChatMessageRoleUser, Content: "刚刚我把 nginx 配置贴出来了。"},
		{Role: openai.ChatMessageRoleAssistant, Content: "我看到了，我们继续定位。"},
	}

	layers := buildConversationContextLayers(conversation, 1, 4, 3)
	if len(layers.RecentTurns) != 1 {
		t.Fatalf("expected 1 recent turn, got %d", len(layers.RecentTurns))
	}

	summary := layers.RollingSummary
	if summary == "" {
		t.Fatal("expected rolling summary")
	}
	if !strings.Contains(summary, "Topics:") {
		t.Fatalf("expected topics in summary, got %q", summary)
	}
	if !strings.Contains(summary, "Facts:") {
		t.Fatalf("expected facts in summary, got %q", summary)
	}
	if !strings.Contains(summary, "Tone: user expressed stress or frustration") {
		t.Fatalf("expected tone in summary, got %q", summary)
	}
	if !strings.Contains(summary, "Commitments:") {
		t.Fatalf("expected commitments in summary, got %q", summary)
	}
	if strings.Contains(summary, "nginx 配置") {
		t.Fatalf("recent turn leaked into rolling summary: %q", summary)
	}
}

func TestExtractOpenLoops(t *testing.T) {
	conversation := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "登录接口还是报错，怎么处理？"},
		{Role: openai.ChatMessageRoleAssistant, Content: "我会稍后把排查步骤整理给你。"},
		{Role: openai.ChatMessageRoleUser, Content: "另一个问题是 webhook 也失败了"},
		{Role: openai.ChatMessageRoleUser, Content: "数据库连接为什么还是超时？"},
	}

	turns := extractConversationTurns(conversation)
	loops := extractOpenLoops(turns, 4)
	if len(loops) != 4 {
		t.Fatalf("expected 4 open loops, got %d: %+v", len(loops), loops)
	}

	if loops[0].Kind != "unresolved_problem" {
		t.Fatalf("expected first loop to be unresolved_problem, got %+v", loops[0])
	}
	if loops[1].Kind != "assistant_follow_up" {
		t.Fatalf("expected second loop to be assistant_follow_up, got %+v", loops[1])
	}
	if loops[2].Kind != "unresolved_problem" {
		t.Fatalf("expected third loop to be unresolved_problem, got %+v", loops[2])
	}
	if loops[3].Kind != "unanswered_question" {
		t.Fatalf("expected fourth loop to be unanswered_question, got %+v", loops[3])
	}
}

func TestStateManagerBuildConversationContextLayers(t *testing.T) {
	sm := &StateManager{
		sessions: map[string]*Session{
			"s1": {
				ID: "s1",
				Conversation: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleUser, Content: "第一个问题怎么做？"},
					{Role: openai.ChatMessageRoleAssistant, Content: "先看日志。"},
					{Role: openai.ChatMessageRoleUser, Content: "第二个问题还没解。"},
				},
			},
		},
	}

	layers := sm.BuildConversationContextLayers("s1", 1, 4, 2)
	if len(layers.RecentTurns) != 1 {
		t.Fatalf("expected one recent turn, got %d", len(layers.RecentTurns))
	}
	if layers.RecentTurns[0].User != "第二个问题还没解。" {
		t.Fatalf("unexpected recent turn: %+v", layers.RecentTurns[0])
	}
	if layers.RollingSummary == "" {
		t.Fatal("expected rolling summary")
	}
	if len(layers.OpenLoops) != 1 || layers.OpenLoops[0].Kind != "unresolved_problem" {
		t.Fatalf("unexpected open loops: %+v", layers.OpenLoops)
	}
}
