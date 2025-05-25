package serve

import (
	"fmt"
	"log"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/global"
	"project-yume/internal/service"

	"github.com/gorilla/websocket"
)

func ResponseUserMsg(c *websocket.Conn, resptext string) (err error) {
	if global.LongChainflag {
		err := ChatwithAi(c, resptext)
		if err != nil {
			return fmt.Errorf("ChatwithAi error: %v", err)
		}
		return nil
	}
	var Emotion string
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
	case "能陪我聊聊吗":
		sendmsg = "好"
		global.LongChainflag = true
	default:
		sendmsg = "?"
		Emotion, err = JudgeEmotion(c, resptext)
		if err != nil {
			return err
		}
		// global.Sleepflag = true
	}
	if global.Sleepflag {
		log.Print("睡眠中")
		time.Sleep(time.Second * 3)
	}

	// 通过情感判断进行不同回应的测试
	if sendmsg == "?" {
		switch Emotion {
		case "开心":
			sendmsg = "那很好了。"
		case "生气":
			sendmsg = "我做错什么了？"
		case "中性":
			sendmsg = "在忙"
		case "哲学":
			sendmsg = "乐"
		default:
			sendmsg = "?"
		}
	}

	err = service.SendMsg(c, config.Config.TargetId, sendmsg)
	if err != nil {
		return err
	}
	return nil
}

func JudgeEmotion(c *websocket.Conn, resptext string) (result string, err error) {
	prompt := "请帮我分析下这段话的情感，并在下面四个选项中选择：开心，生气，中性，哲学， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容"
	resp, err := aifunction.Queryai(prompt, resptext)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func ChatwithAi(c *websocket.Conn, msg string) (err error) {
	if msg == "不聊了" {
		err := service.SendMsg(c, config.Config.TargetId, "好吧")
		if err != nil {
			return fmt.Errorf("SendMsg error in ChatwithAi: %v", err)
		}
		global.LongChainflag = false
		return nil
	}
	filepath := "../../public/aichatlog/longchain/log_" + time.Now().Format("06-01-02") + ".txt"
	Conversation := append(global.Conversation, aifunction.Message{Role: "user", Content: msg})
	log.Print(Conversation)
	NewConversation, result, err := aifunction.QueryaiWithChain(Conversation, filepath)
	if err != nil {
		return err
	}
	err = service.SendMsg(c, config.Config.TargetId, result)
	if err != nil {
		return err
	}
	global.Conversation = NewConversation
	return nil
}
