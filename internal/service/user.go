package service

import (
	"context"
	"fmt"
	"strings"

	"project-yume/internal/config"

	"github.com/gorilla/websocket"
)

func SendMsg(c *websocket.Conn, userID int64, msg string) error {
	result := DeliverReply(context.Background(), c, DeliveryRequest{
		UserID: userID,
		Reply:  msg,
	})
	if result.Status == DeliveryResultDelivered {
		return nil
	}
	if result.Error != "" {
		return fmt.Errorf("reply delivery %s: %s", result.Status, result.Error)
	}
	return fmt.Errorf("reply delivery %s", result.Status)
}

func BuildAssistantTranscript(reply string) string {
	chunks := ParseReplyChunks(reply)
	parts := make([]string, 0, len(chunks))

	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Text); text != "" {
			parts = append(parts, strings.TrimSpace(strings.Join(splitReplySegments(text), " ")))
		}
		if chunk.ImageAssetID != "" {
			parts = append(parts, "[图片]")
		}
	}

	if len(parts) == 0 {
		return strings.TrimSpace(StripReplyDirectives(reply))
	}

	return strings.Join(parts, " ")
}

func splitReplySegments(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	trimmed = normalizeReplyNewlines(trimmed)
	if strings.Contains(trimmed, "$") || strings.Contains(trimmed, "\n\n") {
		return splitReplyDelimiters(trimmed)
	}

	if config.GetConfig().EnableSpaceSegmentDelimiter && strings.Contains(trimmed, " ") {
		return strings.Fields(trimmed)
	}

	return []string{trimmed}
}

func normalizeReplyNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func splitReplyDelimiters(text string) []string {
	text = strings.ReplaceAll(text, "$", "\n\n")
	parts := strings.Split(text, "\n\n")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}
