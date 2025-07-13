package serve

import (
	"log"
	"math/rand"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/global"
	"project-yume/internal/service"

	"github.com/gorilla/websocket"
)

var respmsgs = []string{
	"在干嘛",
	"sad",
	"难过了",
	"鼓励我一下",
}

func HandleScheduled(c *websocket.Conn) {
	// 生成随机数
	rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := rand.Intn(len(respmsgs))
	if idx == 2 || idx == 3 {
		global.Needcomfort = true
	}
	if idx == 4 {
		global.Needencourage = true
	}
	err := service.SendMsg(c, config.GetConfig().TargetId, respmsgs[idx])
	if err != nil {
		log.Println("发送消息失败 in HandleScheduled:", err)
	}
}
