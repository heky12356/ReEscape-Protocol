package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/service"

	"github.com/gorilla/websocket"
)

func main() {
	config.Init()
	c, err := connect.Init(config.Config.Hostadd)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	flag := false // 标记是否回复过
	cnt := 0      // 全局计数器

	// 监听中断信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	// 启动协程读取消息
	go func() {
		defer close(done)
		for {
			t, message, err := c.ReadMessage()
			if err != nil {
				log.Println("读取消息失败:", err)
				return
			}
			log.Printf("接收到消息: %s", message)
			var msg model.Response
			err = json.Unmarshal(message, &msg)
			if err != nil {
				log.Println("消息反序列化失败:", err)
			}
			log.Print(msg)
			if msg.User_id == config.Config.TargetId && msg.Message_type == "private" {
				resptext := msg.Raw_message
				sendmsg := ""
				switch resptext {
				case "你好":
					sendmsg = "你好"
				case "在干嘛":
					sendmsg = "在学习"
				case "在忙呢":
					flag = true
					sendmsg = "好吧"
				}
				err := service.SendMsg(c, config.Config.TargetId, sendmsg)
				if err != nil {
					log.Println("发送消息失败:", err)
				}
			}

			log.Println(t)
		}
	}()

	// 定时发送消息

	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for t := range ticker.C {
			cnt = (cnt + 1) % 32
			log.Println("计数器：", cnt)
			if cnt == 30 {
				flag = false
				log.Println("已1h，重新恢复定时")
			}
			log.Println("定时触发：", t)
			if flag {
				log.Println("已回复，时间修改为60min后")
				continue
			}
			err := service.SendMsg(c, config.Config.TargetId, "在干嘛")
			if err != nil {
				log.Println("发送消息失败:", err)
			}
		}
	}()

	// 主循环，处理中断信号
	for {
		select {
		case <-done:
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
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
