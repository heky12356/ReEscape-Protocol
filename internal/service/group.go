package service

import (
	"encoding/json"

	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func SendGroupMsg(c *websocket.Conn, GroupId int64, msg string) (err error) {
	// fmt.Println("GroupId:", GroupId, "msg:", msg)
	ws_msg := model.Message{
		Action: "send_group_msg",
		Params: model.MessageParams{
			Group_id: GroupId,
			Message:  msg,
		},
		Echo: "send_msg",
	}
	jsonData, err := json.Marshal(ws_msg)
	if err != nil {
		utils.Error("Error marshaling JSON: %v", err)
		return err
	}

	// 通过 WebSocket 发送消息到 OneBot
	err = connect.WriteMessage(c, websocket.TextMessage, jsonData)
	if err != nil {
		utils.Error("Write Error: %v", err)
		return err
	}
	return nil
}

func SendGroupMsgEmoji(c *websocket.Conn, Message_id int64) (err error) {
	ws_msg := model.Message{
		Action: "set_msg_emoji_like",
		Params: model.SetMsgEmoji{
			Message_id: int32(Message_id),
			Emoji_id:   1,
			Set:        true,
		},
		Echo: "set_msg_emoji_like",
	}
	jsonData, err := json.Marshal(ws_msg)
	if err != nil {
		utils.Error("Error marshaling JSON: %v", err)
		return err
	}

	// 通过 WebSocket 发送消息到 OneBot
	err = connect.WriteMessage(c, websocket.TextMessage, jsonData)
	if err != nil {
		utils.Error("Write Error: %v", err)
		return err
	}
	return nil
}
