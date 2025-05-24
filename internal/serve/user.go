package serve

import (
	"log"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/global"
	"project-yume/internal/service"

	"github.com/gorilla/websocket"
)

func ResponseUserMsg(c *websocket.Conn, resptext string) (err error) {
	sendmsg := ""
	switch resptext {
	case "你好":
		sendmsg = "你好"
	case "在干嘛":
		sendmsg = "在学习"
	case "在忙呢":
		global.Flag = true
		sendmsg = "好吧"
	case "我不信":
		sendmsg = "[CQ:image,type=image,url=https://pan.heky.top/photo/v2-58816628de7a7812f1afd46fd411090c_b.jpg,title=image]"
	default:
		sendmsg = "?"
		global.Sleepflag = true
	}
	if global.Sleepflag {
		log.Print("睡眠中")
		time.Sleep(time.Second * 3)
	}
	err = service.SendMsg(c, config.Config.TargetId, sendmsg)
	if err != nil {
		return err
	}
	return nil
}
