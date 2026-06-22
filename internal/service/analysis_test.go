package service

import "testing"

func TestParseMessageAnalysisStructuredReply(t *testing.T) {
	raw := `{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"full_reply","reply_expectation":"high","turn_status":"handoff_to_ai","support_strategy":"continue_chat","topic":"今天的安排","user_need":"被回应","confidence":0.88,"visible_reply":"那你准备先做哪件事？"}`

	result, err := parseMessageAnalysis(raw, AnalysisModeDefault)
	if err != nil {
		t.Fatalf("parseMessageAnalysis returned error: %v", err)
	}
	if result.ReplyMode != ReplyModeFullReply {
		t.Fatalf("unexpected reply mode: %q", result.ReplyMode)
	}
	if result.VisibleReply == "" {
		t.Fatalf("expected visible reply to be populated")
	}
}

func TestParseMessageAnalysisRejectsVisibleReplyForNoReply(t *testing.T) {
	raw := `{"emotion":"中性","intention":"想和对方聊天","wanna_bye":"想继续","reply_mode":"no_reply","reply_expectation":"low","turn_status":"user_holds_floor","support_strategy":"acknowledge_and_wait","topic":"补充观点","user_need":"继续表达","confidence":0.62,"visible_reply":"嗯"}`

	if _, err := parseMessageAnalysis(raw, AnalysisModeDefault); err == nil {
		t.Fatalf("expected parseMessageAnalysis to reject visible_reply when reply_mode=no_reply")
	}
}
