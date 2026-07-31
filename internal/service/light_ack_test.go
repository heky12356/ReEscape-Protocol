package service

import (
	"testing"

	"project-yume/internal/config"
)

func TestSelectLightAckPrefersLLMReplyByDefault(t *testing.T) {
	cfg := config.GetConfig()
	previousMode := cfg.LightAckMode
	cfg.LightAckMode = "llm"
	defer func() {
		cfg.LightAckMode = previousMode
	}()

	analysis := MessageAnalysis{
		WannaBye:        "想继续",
		TurnStatus:      "user_holds_floor",
		SupportStrategy: "acknowledge_and_wait",
		UserNeed:        "继续表达",
		VisibleReply:    "你继续说",
	}

	reply := SelectLightAck("session-0", "然后我又想了一下", analysis)
	if reply != "你继续说" {
		t.Fatalf("expected llm light ack, got %q", reply)
	}
}

func TestSelectLightAckListeningPool(t *testing.T) {
	cfg := config.GetConfig()
	previousMode := cfg.LightAckMode
	cfg.LightAckMode = "template"
	defer func() {
		cfg.LightAckMode = previousMode
	}()

	analysis := MessageAnalysis{
		WannaBye:        "想继续",
		TurnStatus:      "user_holds_floor",
		SupportStrategy: "acknowledge_and_wait",
		UserNeed:        "继续表达",
	}

	reply := SelectLightAck("session-1", "然后我又想了一下", analysis)
	if reply != "嗯嗯" && reply != "然后呢然后呢" {
		t.Fatalf("unexpected listening ack: %q", reply)
	}
}

func TestSelectLightAckClosePool(t *testing.T) {
	cfg := config.GetConfig()
	previousMode := cfg.LightAckMode
	cfg.LightAckMode = "template"
	defer func() {
		cfg.LightAckMode = previousMode
	}()

	analysis := MessageAnalysis{
		WannaBye:        "想结束对话",
		TurnStatus:      "handoff_to_ai",
		SupportStrategy: "close_conversation",
	}

	reply := SelectLightAck("session-2", "晚安", analysis)
	if reply != "好好，晚安。" && reply != "嗯嗯，拜拜。" && reply != "好噢" && reply != "彳亍" {
		t.Fatalf("unexpected close ack: %q", reply)
	}
}

func TestSelectLightAckStableSelection(t *testing.T) {
	cfg := config.GetConfig()
	previousMode := cfg.LightAckMode
	cfg.LightAckMode = "template"
	defer func() {
		cfg.LightAckMode = previousMode
	}()

	analysis := MessageAnalysis{
		WannaBye:        "想继续",
		TurnStatus:      "handoff_to_ai",
		SupportStrategy: "continue_chat",
		UserNeed:        "被回应",
	}

	first := SelectLightAck("session-3", "是这样", analysis)
	second := SelectLightAck("session-3", "是这样", analysis)
	if first != second {
		t.Fatalf("expected stable selection, got %q and %q", first, second)
	}
}
