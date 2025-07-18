package serve

import (
	"log"
	"math/rand"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/scheduler"
	"project-yume/internal/service"
	"project-yume/internal/state"

	"github.com/gorilla/websocket"
)

var respmsgs = []string{
	"在干嘛",
	"sad",
	"难过了",
	"鼓励我一下",
}

// LegacyHandleScheduled 保持向后兼容的定时任务处理
func LegacyHandleScheduled(c *websocket.Conn) {
	sm := state.GetManager()

	// 生成随机数
	rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := rand.Intn(len(respmsgs))

	// 根据消息类型设置状态
	if idx == 2 || idx == 3 {
		sm.SetFlag(state.FlagNeedComfort, true)
	}
	if idx == 4 {
		sm.SetFlag(state.FlagNeedEncourage, true)
	}

	err := service.SendMsg(c, config.GetConfig().TargetId, respmsgs[idx])
	if err != nil {
		log.Println("发送消息失败 in LegacyHandleScheduled:", err)
	}
}

// HandleScheduled 新的定时任务处理函数
func HandleScheduled(c *websocket.Conn) {
	cfg := config.GetConfig()

	// 如果启用了自然定时器，使用新的调度器
	if cfg.EnableNaturalScheduler {
		naturalScheduler := scheduler.NewNaturalScheduler()
		err := naturalScheduler.SendScheduledMessage(c)
		if err != nil {
			log.Printf("自然定时器发送消息失败: %v", err)
			// 降级到传统方式
			LegacyHandleScheduled(c)
		}
	} else {
		// 使用传统方式
		LegacyHandleScheduled(c)
	}
}
