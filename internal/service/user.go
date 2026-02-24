package service

import (
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func SendMsg(c *websocket.Conn, User_id int64, msg string) (err error) {
	// fmt.Println("GroupId:", GroupId, "msg:", msg)
	var msgs []string = strings.Split(msg, "$")

	for _, m := range msgs {
		ws_msg := model.Message{
			Action: "send_private_msg",
			Params: model.UserMessageParams{
				User_id: User_id,
				Message: m,
			},
			Echo: "send_msg",
		}
		jsonData, err := json.Marshal(ws_msg)
		if err != nil {
			utils.Error("Error marshaling JSON: %v", err)
			return err
		}

		// 随机延迟 1-3 秒
		time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)

		// 通过 WebSocket 发送消息到 OneBot
		err = connect.WriteMessage(c, websocket.TextMessage, jsonData)
		if err != nil {
			utils.Error("Write Error: %v", err)
			return err
		}
	}
	return nil
}
