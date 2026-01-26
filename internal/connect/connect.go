package connect

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"

	"project-yume/internal/config"
	"project-yume/internal/utils"
)

func Init(host string) (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	utils.Info("连接到 %s", u.String())

	header := http.Header{}
	header.Set("Authorization", "Bearer "+config.GetConfig().Token)

	// 建立 WebSocket 连接
	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		utils.Error("连接失败: %v", err)
		return nil, err
	}
	return c, nil
}
