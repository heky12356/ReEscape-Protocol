package service

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"project-yume/internal/assets"
	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func SendMsg(c *websocket.Conn, userID int64, msg string) error {
	chunks := ParseReplyChunks(msg)
	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Text); text != "" {
			for _, segment := range splitReplySegments(text) {
				trimmed := strings.TrimSpace(segment)
				if trimmed == "" {
					continue
				}
				if err := sendPrivateRawMessage(c, userID, trimmed); err != nil {
					return err
				}
			}
		}

		if chunk.ImageAssetID != "" {
			if err := sendPrivateImageAsset(c, userID, chunk.ImageAssetID); err != nil {
				return fmt.Errorf("send image asset %q: %w", chunk.ImageAssetID, err)
			}
		}
	}
	return nil
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

func sendPrivateImageAsset(c *websocket.Conn, userID int64, assetID string) error {
	asset, err := assets.LookupImageAsset(assetID)
	if err != nil {
		return err
	}

	fileValue, err := assets.ResolveImageAssetCQFile(asset)
	if err != nil {
		return err
	}

	return sendPrivateRawMessage(c, userID, fmt.Sprintf("[CQ:image,file=%s]", fileValue))
}

func sendPrivateRawMessage(c *websocket.Conn, userID int64, msg string) error {
	time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
	_, err := connect.CallAPI(c, "send_private_msg", model.UserMessageParams{
		User_id: userID,
		Message: msg,
	})
	if err != nil {
		utils.Error("send private message failed: %v", err)
		return err
	}
	return nil
}
