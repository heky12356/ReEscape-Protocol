package app

import (
	"context"
	"encoding/json"
	"html"
	"strings"
	"time"

	"project-yume/internal/connect"
	"project-yume/internal/metrics"
	"project-yume/internal/model"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func startMessageReceiver(c *websocket.Conn, msgChan chan model.Msg, ctx context.Context) {
	defer close(msgChan)

	for {
		select {
		case <-ctx.Done():
			utils.Info("消息接收器已停止")
			return
		default:
			_, message, err := c.ReadMessage()
			if err != nil {
				utils.Error("读取消息失败: %v", err)
				time.Sleep(time.Second)
				continue
			}

			utils.Info("接收到消息: %s", message)

			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(message, &envelope); err == nil {
				if _, hasPostType := envelope["post_type"]; !hasPostType {
					if connect.DispatchAPIResponse(message) {
						continue
					}
				}
			}

			metrics.IncCounter(
				"bot_ws_messages_total",
				"Total WebSocket messages by lifecycle result.",
				map[string]string{"result": "received"},
			)

			var msg model.Response
			err = json.Unmarshal(message, &msg)
			if err != nil {
				utils.Error("消息反序列化失败: %v", err)
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "decode_error"},
				)
				continue
			}
			state.GetManager().CancelActiveDelivery(state.BuildSessionID(msg.User_id, msg.Group_id, func() int {
				if msg.Message_type == "group" {
					return 0
				}
				return 1
			}()))

			internalMsg := model.Msg{
				Message:   msg.Raw_message,
				Parts:     buildIncomingMessageParts(msg),
				User_id:   msg.User_id,
				Group_id:  msg.Group_id,
				MessageID: msg.Message_id,
				Time:      msg.Time,
				Type: func() int {
					if msg.Message_type == "group" {
						return 0
					}
					return 1
				}(),
			}

			select {
			case msgChan <- internalMsg:
			case <-time.After(100 * time.Millisecond):
				utils.Warn("消息通道满，丢弃消息")
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "channel_dropped"},
				)
			}
		}
	}
}

func buildIncomingMessageParts(resp model.Response) []model.MessagePart {
	parts := make([]model.MessagePart, 0, len(resp.Message))

	for _, segment := range resp.Message {
		switch segment.Type {
		case "text":
			text := decodeIncomingSegmentValue(segment.Data["text"])
			if text == "" {
				continue
			}
			parts = append(parts, model.MessagePart{
				Type: "text",
				Text: text,
			})
		case "image":
			parts = append(parts, model.MessagePart{
				Type: "image",
				URL:  decodeIncomingSegmentValue(segment.Data["url"]),
				File: decodeIncomingSegmentValue(segment.Data["file"]),
			})
		}
	}

	if len(parts) > 0 {
		return parts
	}

	raw := strings.TrimSpace(resp.Raw_message)
	if raw == "" {
		return nil
	}
	if utils.IsCQImage(raw) {
		return []model.MessagePart{{
			Type: "image",
			URL:  strings.TrimSpace(utils.ExtractImageURL(raw)),
			File: strings.TrimSpace(utils.ExtractImageFile(raw)),
		}}
	}
	return []model.MessagePart{{
		Type: "text",
		Text: raw,
	}}
}

func decodeIncomingSegmentValue(value json.RawMessage) string {
	var decoded string
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	if err := json.Unmarshal(value, &decoded); err != nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(decoded))
}
