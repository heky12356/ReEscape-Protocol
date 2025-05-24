package serve

import (
	"project-yume/internal/config"
	"project-yume/internal/service"

	"github.com/gorilla/websocket"
)

func ResponseUserMsg(c *websocket.Conn, msg string) (err error) {
	err = service.SendMsg(c, config.Config.TargetId, msg)
	if err != nil {
		return err
	}
	return nil
}
