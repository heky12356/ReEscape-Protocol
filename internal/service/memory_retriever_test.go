package service

import (
	"strings"
	"testing"

	"project-yume/internal/state"
)

func TestFormatPromptMemoryIncludesLayeredShortTermContext(t *testing.T) {
	context := FormatPromptMemory(PromptMemory{
		ContextLayers: state.ConversationContextLayers{
			RecentTurns: []state.ConversationTurn{
				{
					User:      "我今天在排查登录问题",
					Assistant: "先看错误日志",
					HasReply:  true,
				},
			},
			RollingSummary: "Topics: 登录问题 | Tone: user expressed stress or frustration",
			OpenLoops: []state.OpenLoop{
				{Kind: "unresolved_problem", Description: "登录接口还是报错"},
			},
		},
	})

	required := []string{
		"【短期上下文】",
		"最近回合：",
		"user: 我今天在排查登录问题",
		"assistant: 先看错误日志",
		"滚动摘要：Topics: 登录问题",
		"未闭合事项：",
		"- [待解决问题] 登录接口还是报错",
	}
	for _, item := range required {
		if !strings.Contains(context, item) {
			t.Fatalf("expected context to contain %q, got %q", item, context)
		}
	}
}
