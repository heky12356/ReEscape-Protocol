package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func SendMsg(c *websocket.Conn, userID int64, msg string) error {
	chunks := ParseReplyChunks(msg)
	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Text); text != "" {
			for _, segment := range strings.Split(text, "$") {
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
				utils.Warn("send image asset failed: %v", err)
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
			parts = append(parts, strings.TrimSpace(strings.ReplaceAll(text, "$", " ")))
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

func sendPrivateImageAsset(c *websocket.Conn, userID int64, assetID string) error {
	asset, err := LookupImageAsset(assetID)
	if err != nil {
		return err
	}

	fileValue, err := ResolveImageAssetCQFile(asset)
	if err != nil {
		return err
	}

	return sendPrivateRawMessage(c, userID, fmt.Sprintf("[CQ:image,file=%s]", fileValue))
}

func sendPrivateRawMessage(c *websocket.Conn, userID int64, msg string) error {
	wsMsg := model.Message{
		Action: "send_private_msg",
		Params: model.UserMessageParams{
			User_id: userID,
			Message: msg,
		},
		Echo: "send_msg",
	}
	jsonData, err := json.Marshal(wsMsg)
	if err != nil {
		utils.Error("Error marshaling JSON: %v", err)
		return err
	}

	time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
	err = connect.WriteMessage(c, websocket.TextMessage, jsonData)
	if err != nil {
		utils.Error("Write Error: %v", err)
		return err
	}
	return nil
}
