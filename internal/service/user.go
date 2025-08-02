package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"project-yume/internal/model"

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
			fmt.Println("Error marshaling JSON:", err)
			return err
		}

		// 随机延迟 1-3 秒
		time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)

		// 通过 WebSocket 发送消息到 OneBot
		err = c.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			fmt.Println("Write Error:", err)
			return err
		}
	}
	return nil
}
