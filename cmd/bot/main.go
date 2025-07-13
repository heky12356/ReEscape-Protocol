package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/global"
	"project-yume/internal/model"
	"project-yume/internal/serve"

	"github.com/gorilla/websocket"
)

func main() {
	c, err := connect.Init(config.GetConfig().Hostadd)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// 监听中断信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// 定义上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 定义channel
	ch := make(chan model.Msg)

	// 启动协程读取消息
	go func(ch chan model.Msg) {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("读取消息失败:", err)
				continue
			}
			log.Printf("接收到消息: %s", message)
			var msg model.Response
			err = json.Unmarshal(message, &msg)
			if err != nil {
				log.Println("消息反序列化失败:", err)
				continue
			}

			// 将消息存储到上下文
			ch <- model.Msg{
				Message:  msg.Raw_message,
				User_id:  msg.User_id,
				Group_id: msg.Group_id,
				Time:     msg.Time,
				Type: func() int {
					if msg.Message_type == "group" {
						return 0
					}
					return 1
				}(),
			}
		}
	}(ch)

	go func(ch chan model.Msg) {
		for {
			for msg := range ch {
				if msg.User_id == config.GetConfig().TargetId && msg.Type == 1 {
					if msg.Message == "exit();" {
						cancel()
					}
					err := serve.ResponseUserMsg(c, msg.Message)
					if err != nil {
						log.Println("使用serve发送消息失败:", err)
					}
					global.ExplainStatus()
				}
			}
		}
	}(ch)
	// 定时发送消息
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for t := range ticker.C {
			global.Count = (global.Count + 1) % 32
			log.Println("计数器：", global.Count)
			if global.Count == 30 {
				global.Flag = false
				log.Println("已1h，重新恢复定时")
			}
			log.Println("定时触发：", t)
			if global.Flag {
				log.Println("已回复，时间修改为60min后")
				continue
			}
			if global.LongChainflag {
				log.Println("正在进行长对话，不回复")
				continue
			}
			serve.HandleScheduled(c)

		}
	}()

	// 主循环，处理中断信号
	for {
		select {
		case <-ctx.Done():
			return
		case <-interrupt:
			log.Println("接收到中断信号，关闭连接")
			// 发送关闭消息
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("发送关闭消息失败:", err)
				return
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			return
		}
	}
}
