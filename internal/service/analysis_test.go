package service

import (
	"strings"
	"testing"

	"project-yume/internal/config"
)

func TestParseMessageAnalysisStructuredReply(t *testing.T) {
	raw := `{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"full_reply","reply_expectation":"high","turn_status":"handoff_to_ai","support_strategy":"continue_chat","topic":"今天的安排","user_need":"被回应","confidence":0.88,"visible_reply":""}`

	result, err := parseMessageAnalysis(raw, AnalysisModeDefault)
	if err != nil {
		t.Fatalf("parseMessageAnalysis returned error: %v", err)
	}
	if result.ReplyMode != ReplyModeFullReply {
		t.Fatalf("unexpected reply mode: %q", result.ReplyMode)
	}
	if result.VisibleReply != "" {
		t.Fatalf("expected visible reply to be empty for full reply, got %q", result.VisibleReply)
	}
}

func TestParseMessageAnalysisRejectsVisibleReplyForNoReply(t *testing.T) {
	raw := `{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"no_reply","reply_expectation":"low","turn_status":"user_holds_floor","support_strategy":"acknowledge_and_wait","topic":"补充观点","user_need":"继续表达","confidence":0.62,"visible_reply":"嗯"}`

	if _, err := parseMessageAnalysis(raw, AnalysisModeDefault); err == nil {
		t.Fatalf("expected parseMessageAnalysis to reject visible_reply when reply_mode=no_reply")
	}
}

func TestRebalanceMessageAnalysisPromotesQuestionToFullReply(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.ShortReplyStrictness
	cfg.ShortReplyStrictness = "balanced"
	defer func() {
		cfg.ShortReplyStrictness = previous
	}()

	result := rebalanceMessageAnalysis("你觉得我现在该怎么办？", MessageAnalysis{
		ReplyMode:        ReplyModeLightAck,
		ReplyExpectation: "medium",
		TurnStatus:       "user_holds_floor",
		SupportStrategy:  "acknowledge_and_wait",
		VisibleReply:     "嗯",
	}, AnalysisModeDefault)

	if result.ReplyMode != ReplyModeFullReply {
		t.Fatalf("expected full reply, got %q", result.ReplyMode)
	}
	if result.VisibleReply != "" {
		t.Fatalf("expected visible reply to be cleared, got %q", result.VisibleReply)
	}
	if result.TurnStatus != "handoff_to_ai" {
		t.Fatalf("expected handoff_to_ai, got %q", result.TurnStatus)
	}
}

func TestRebalanceMessageAnalysisKeepsContinuationLightAck(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.ShortReplyStrictness
	cfg.ShortReplyStrictness = "balanced"
	defer func() {
		cfg.ShortReplyStrictness = previous
	}()

	result := rebalanceMessageAnalysis("然后我后来又想了一下，", MessageAnalysis{
		ReplyMode:        ReplyModeLightAck,
		ReplyExpectation: "medium",
		TurnStatus:       "user_holds_floor",
		SupportStrategy:  "acknowledge_and_wait",
		VisibleReply:     "嗯",
	}, AnalysisModeLongChat)

	if result.ReplyMode != ReplyModeLightAck {
		t.Fatalf("expected light ack, got %q", result.ReplyMode)
	}
}

func TestRebalanceMessageAnalysisReplyFirstPromotesMediumExpectation(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.ShortReplyStrictness
	cfg.ShortReplyStrictness = "reply_first"
	defer func() {
		cfg.ShortReplyStrictness = previous
	}()

	result := rebalanceMessageAnalysis("我今天其实挺纠结这件事", MessageAnalysis{
		ReplyMode:        ReplyModeLightAck,
		ReplyExpectation: "medium",
		TurnStatus:       "handoff_to_ai",
		SupportStrategy:  "continue_chat",
		VisibleReply:     "嗯",
	}, AnalysisModeDefault)

	if result.ReplyMode != ReplyModeFullReply {
		t.Fatalf("expected full reply under reply_first, got %q", result.ReplyMode)
	}
}

func TestRebalanceMessageAnalysisConservativeKeepsLightAck(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.ShortReplyStrictness
	cfg.ShortReplyStrictness = "conservative"
	defer func() {
		cfg.ShortReplyStrictness = previous
	}()

	result := rebalanceMessageAnalysis("我今天其实挺纠结这件事", MessageAnalysis{
		ReplyMode:        ReplyModeLightAck,
		ReplyExpectation: "medium",
		TurnStatus:       "handoff_to_ai",
		SupportStrategy:  "continue_chat",
		VisibleReply:     "嗯",
	}, AnalysisModeDefault)

	if result.ReplyMode != ReplyModeLightAck {
		t.Fatalf("expected light ack under conservative, got %q", result.ReplyMode)
	}
}

func TestFallbackAnalysisDefaultsToFullReply(t *testing.T) {
	result := fallbackAnalysis(AnalysisModeDefault)
	if result.ReplyMode != ReplyModeFullReply {
		t.Fatalf("expected full reply fallback, got %q", result.ReplyMode)
	}
	if result.VisibleReply != "" {
		t.Fatalf("expected empty visible reply fallback, got %q", result.VisibleReply)
	}
}

func TestBuildAnalysisPromptUsesToneSummaryNotEffectivePrompt(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.AiPrompt
	cfg.AiPrompt = "SENTINEL_FULL_EFFECTIVE_PROMPT_SHOULD_NOT_APPEAR"
	defer func() {
		cfg.AiPrompt = previous
	}()

	prompt := buildAnalysisPrompt(AnalysisInput{Mode: AnalysisModeDefault})
	if strings.Contains(prompt, "SENTINEL_FULL_EFFECTIVE_PROMPT_SHOULD_NOT_APPEAR") {
		t.Fatalf("analysis prompt should not include full effective prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "角色语气摘要") {
		t.Fatalf("analysis prompt should include character tone summary: %q", prompt)
	}
	if !strings.Contains(prompt, "不得影响 emotion/intention/reply_mode 等客观字段判断") {
		t.Fatalf("analysis prompt should keep objective fields separated: %q", prompt)
	}
}
