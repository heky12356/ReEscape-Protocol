package connect

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"

	"project-yume/internal/config"
)

func Init(host string) (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	log.Printf("连接到 %s", u.String())

	header := http.Header{}
	header.Set("Authorization", "Bearer "+config.GetConfig().Token)

	// 建立 WebSocket 连接
	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Println("连接失败:", err)
		return nil, err
	}
	return c, nil
}
