package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/global"
	"project-yume/internal/model"
	"project-yume/internal/serve"
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
			// log.Print(msg)
			if msg.User_id == config.Config.TargetId && msg.Message_type == "private" {

				// if !global.Aiflag && msg.Raw_message[0] != '/' {
				// 	result, err := serve.JudgeEmotion(c, msg.Raw_message)
				// 	if err != nil {
				// 		log.Println("判断情感失败:", err)
				// 	}
				// 	log.Print("情感：" + result)
				// }
				if global.Aiflag && msg.Raw_message[0] != '/' {
					resp, err := aifunction.Queryai(config.Config.AiPrompt, msg.Raw_message)
					if err != nil {
						log.Println("使用ai发送消息失败:", err)
					}
					err = service.SendMsg(c, config.Config.TargetId, resp)
					if err != nil {
						log.Println("使用serve发送消息失败:", err)
					}
				}

				if !global.Aiflag && msg.Raw_message[0] != '/' {
					err := serve.ResponseUserMsg(c, msg.Raw_message)
					if err != nil {
						log.Println("使用serve发送消息失败:", err)
					}
				}

				if msg.Raw_message == "/test" {
					global.Aiflag = true
				}
				if msg.Raw_message == "/end" {
					global.Aiflag = false
				}
			}

			log.Println(t)
		}
	}()

	// 定时发送消息

	ticker := time.NewTicker(2 * time.Minute)
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
