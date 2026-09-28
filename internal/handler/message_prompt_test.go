package handler

import (
	"strings"
	"testing"

	"project-yume/internal/config"
	"project-yume/internal/service"

	"github.com/sashabaranov/go-openai"
)

func TestBuildReplyOutputContractReinforcesDollarSegmentation(t *testing.T) {
	contract := buildReplyOutputContract(service.MessageAnalysis{
		ReplyExpectation: "high",
		SupportStrategy:  "answer_directly",
	})

	required := []string{
		"【最终输出要求】",
		"优先使用 $ 作为分段标记",
		"连续两个换行分段",
		"输出示例：第一句$第二句$第三句 或 第一句\n\n第二句\n\n第三句",
		"最后自检：只要不是单句短回应，就必须明确使用 $ 或连续两个换行分段后再输出。",
	}
	for _, item := range required {
		if !strings.Contains(contract, item) {
			t.Fatalf("expected contract to contain %q, got %q", item, contract)
		}
	}
}

func TestBuildGenerationSystemPromptEndsWithOutputContract(t *testing.T) {
	prompt := buildGenerationSystemPrompt(MessageContext{}, service.MessageAnalysis{
		ReplyExpectation: "high",
		SupportStrategy:  "continue_chat",
		Topic:            "今天的安排",
		UserNeed:         "被回应",
		TurnStatus:       "handoff_to_ai",
		WannaBye:         "想继续",
	})

	if !strings.Contains(prompt, "【当前回复决策】") {
		t.Fatalf("expected decision guidance in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "【最终输出要求】") {
		t.Fatalf("expected output contract in prompt, got %q", prompt)
	}
	if !strings.HasSuffix(strings.TrimSpace(prompt), "最后自检：只要不是单句短回应，就必须明确使用 $ 或连续两个换行分段后再输出。") {
		t.Fatalf("expected prompt to end with hard output contract, got %q", prompt)
	}
}

func TestBuildReplyOutputContractAllowsSpaceDelimiterWhenEnabled(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = true
	defer func() {
		cfg.EnableSpaceSegmentDelimiter = previous
	}()

	contract := buildReplyOutputContract(service.MessageAnalysis{
		ReplyExpectation: "high",
		SupportStrategy:  "answer_directly",
	})

	required := []string{
		"优先使用 $ 作为分段标记",
		"或使用空格隔开各段",
		"输出示例：第一句$第二句$第三句 或 第一句\n\n第二句\n\n第三句 或 第一句 第二句 第三句",
	}
	for _, item := range required {
		if !strings.Contains(contract, item) {
			t.Fatalf("expected contract to contain %q, got %q", item, contract)
		}
	}
}

func TestSelectRecentTurnMessagesKeepsLastUserTurns(t *testing.T) {
	conversation := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "old system"},
		{Role: openai.ChatMessageRoleUser, Content: "第一轮"},
		{Role: openai.ChatMessageRoleAssistant, Content: "第一答"},
		{Role: openai.ChatMessageRoleUser, Content: "第二轮"},
		{Role: openai.ChatMessageRoleAssistant, Content: "第二答"},
		{Role: openai.ChatMessageRoleUser, Content: "第三轮"},
	}

	selected := selectRecentTurnMessages(conversation, 2)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected messages, got %d: %+v", len(selected), selected)
	}
	if selected[0].Role != openai.ChatMessageRoleUser || selected[0].Content != "第二轮" {
		t.Fatalf("expected selection to start at second user turn, got %+v", selected[0])
	}
	if selected[2].Role != openai.ChatMessageRoleUser || selected[2].Content != "第三轮" {
		t.Fatalf("expected unmatched current user turn to be kept, got %+v", selected[2])
	}
}

func TestEnsureSystemPromptReplacesOnlyPromptCopySystem(t *testing.T) {
	conversation := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "old system"},
		{Role: openai.ChatMessageRoleUser, Content: "第一轮"},
	}

	selected := selectRecentTurnMessages(conversation, 8)
	promptConversation := ensureSystemPrompt(selected, "new system")

	if len(promptConversation) != 2 {
		t.Fatalf("expected system plus one user message, got %d: %+v", len(promptConversation), promptConversation)
	}
	if promptConversation[0].Role != openai.ChatMessageRoleSystem || promptConversation[0].Content != "new system" {
		t.Fatalf("unexpected prompt system message: %+v", promptConversation[0])
	}
	if promptConversation[1].Role != openai.ChatMessageRoleUser || promptConversation[1].Content != "第一轮" {
		t.Fatalf("unexpected prompt user message: %+v", promptConversation[1])
	}
}
