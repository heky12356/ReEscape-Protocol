package service

import (
	"encoding/json"
	"fmt"

	"project-yume/internal/model"

	"github.com/gorilla/websocket"
)

func SendMsg(c *websocket.Conn, User_id int64, msg string) (err error) {
	// fmt.Println("GroupId:", GroupId, "msg:", msg)
	ws_msg := model.Message{
		Action: "send_private_msg",
		Params: model.UserMessageParams{
			User_id: User_id,
			Message: msg,
		},
		Echo: "send_msg",
	}
	jsonData, err := json.Marshal(ws_msg)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	// 通过 WebSocket 发送消息到 OneBot
	err = c.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		fmt.Println("Write Error:", err)
		return err
	}
	return nil
}
