package handler

import (
	"strings"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/model"
	"project-yume/internal/service"

	"github.com/sashabaranov/go-openai"
)

// MessageContext is the normalized input shared by the inbound pipeline and ReAct runtime.
type MessageContext struct {
	RequestID                  string
	SessionID                  string
	UserID                     int64
	GroupID                    int64
	ChatType                   int
	MessageID                  int64
	MessageIDs                 []int64
	RawSegments                []string
	RawSegmentTimes            []int64
	Parts                      []model.MessagePart
	Aggregated                 bool
	SegmentCount               int
	RawMessage                 string
	Message                    string
	ReceivedAt                 time.Time
	StartedAt                  time.Time
	EndedAt                    time.Time
	PreviousUserMessageAt      time.Time
	PreviousAssistantMessageAt time.Time
	PreviousInteractionAt      time.Time
	DropReason                 string
}

// ProcessResult is the normalized outcome of a ReAct turn.
type ProcessResult struct {
	Handled         bool
	Replied         bool
	MemoryManaged   bool
	ScheduleManaged bool
	Emotion         string
	Intention       string
	ReplyMode       service.ReplyMode
	Reply           string
}

func BuildConversationUserMessage(ctx MessageContext) (openai.ChatCompletionMessage, bool) {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.EnableVisionInput {
		return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: ctx.Message}, true
	}

	parts := make([]openai.ChatMessagePart, 0, len(ctx.Parts)+1)
	if strings.TrimSpace(ctx.Message) != "" {
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: ctx.Message,
		})
	}

	detail := normalizeVisionImageDetail(cfg.VisionImageDetail)
	for _, part := range ctx.Parts {
		if part.Type != "image" || strings.TrimSpace(part.URL) == "" {
			continue
		}
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    strings.TrimSpace(part.URL),
				Detail: detail,
			},
		})
	}

	if len(parts) == 0 {
		return openai.ChatCompletionMessage{}, false
	}
	if len(parts) == 1 && parts[0].Type == openai.ChatMessagePartTypeText {
		return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: parts[0].Text}, true
	}
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, MultiContent: parts}, true
}

func normalizeVisionImageDetail(raw string) openai.ImageURLDetail {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(openai.ImageURLDetailHigh):
		return openai.ImageURLDetailHigh
	case string(openai.ImageURLDetailLow):
		return openai.ImageURLDetailLow
	default:
		return openai.ImageURLDetailAuto
	}
}
