package handler

import (
	"testing"

	"project-yume/internal/config"
	"project-yume/internal/model"
)

func TestBuildConversationUserMessageIncludesVisionParts(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableVisionInput
	cfg.EnableVisionInput = true
	defer func() { cfg.EnableVisionInput = previous }()

	message, ok := BuildConversationUserMessage(MessageContext{
		Message: "看这个",
		Parts:   []model.MessagePart{{Type: "image", URL: "https://example.com/image.png"}},
	})
	if !ok || len(message.MultiContent) != 2 {
		t.Fatalf("expected text and image content, got ok=%t message=%+v", ok, message)
	}
}
