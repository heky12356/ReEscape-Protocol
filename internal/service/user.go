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

	if strings.Contains(trimmed, "$") {
		return strings.Split(trimmed, "$")
	}

	if config.GetConfig().EnableSpaceSegmentDelimiter && strings.Contains(trimmed, " ") {
		return strings.Fields(trimmed)
	}

	return []string{trimmed}
}
