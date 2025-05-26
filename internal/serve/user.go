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
	var Intention string

	if global.PerfunctoryFlag {
		Intention, err = JudgeIntention(c, resptext)
		if err != nil {
			return err
		}
		Emotion, err = JudgeEmotion(c, resptext)
		if err != nil {
			return err
		}
		if Emotion == "生气" {
			err := service.SendMsg(c, config.Config.TargetId, "别生气")
			if err != nil {
				return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
			}
			global.PerfunctoryFlag = false
			return nil
		}
		if Intention == "和对方道歉" {
			err := service.SendMsg(c, config.Config.TargetId, "没事，我没生气")
			if err != nil {
				return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
			}
			err = service.SendMsg(c, config.Config.TargetId, "真的")
			if err != nil {
				return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
			}
			global.PerfunctoryFlag = false
			return nil
		}
	}

	if global.Needencourage {
		if !global.Encourageflag {
			Intention, err = JudgeIntention(c, resptext)
			if err != nil {
				return err
			}
			Emotion, err = JudgeEmotion(c, resptext)
			if err != nil {
				return err
			}
			if Emotion == "敷衍" {
				err := service.SendMsg(c, config.Config.TargetId, "敷衍")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				global.PerfunctoryFlag = true
				return nil
			}
			if Intention == "鼓励对方" {
				err := service.SendMsg(c, config.Config.TargetId, "嘿嘿")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				err = service.SendMsg(c, config.Config.TargetId, "谢谢")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				global.Encourageflag = true
				return nil
			} else {
				err := service.SendMsg(c, config.Config.TargetId, "快鼓励我！！")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				return nil
			}
		} else {
			global.Needencourage = false
		}
	}

	if global.Needcomfort {
		if !global.Comfortflag {
			Intention, err = JudgeIntention(c, resptext)
			if err != nil {
				return err
			}
			Emotion, err = JudgeEmotion(c, resptext)
			if err != nil {
				return err
			}
			if Emotion == "敷衍" {
				err := service.SendMsg(c, config.Config.TargetId, "敷衍")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				global.PerfunctoryFlag = true
				return nil
			}
			if Intention == "安慰对方" {
				err := service.SendMsg(c, config.Config.TargetId, "嘿嘿")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				err = service.SendMsg(c, config.Config.TargetId, "谢谢")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				global.Comfortflag = true
				return nil
			} else {
				err := service.SendMsg(c, config.Config.TargetId, "sad")
				if err != nil {
					return fmt.Errorf("SendMsg error in ResponseUserMsg: %v", err)
				}
				return nil
			}
		} else {
			global.Needcomfort = false
		}
	}

	sendmsg := ""
	switch resptext {
	case "你好":
		sendmsg = "你好"
	case "在干嘛":
		sendmsg = "在学习"
	case "在忙呢":
		global.Flag = true
		sendmsg = "好吧"
	case "难过了":
		sendmsg = "别难过，开心点，加油！"
	case "我不信":
		sendmsg = "[CQ:image,type=image,url=https://pan.heky.top/photo/v2-58816628de7a7812f1afd46fd411090c_b.jpg,title=image]"
	case "我想你了":
		sendmsg = "是嘛？嘿嘿"
	case "能陪我聊聊吗":
		sendmsg = "好"
		global.Conversation = append(global.Conversation, aifunction.Message{Role: "user", Content: "能陪我聊聊吗"}, aifunction.Message{Role: "assistant", Content: "好"})
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

	tmpflag := false // 标记是否需要进行长对话判断，还在思考这里的逻辑要如何才比较好。

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
		case "难过":
			sendmsg, _ = Generate("sad")
		default:
			sendmsg = "?"
			tmpflag = true
		}
	}

	if tmpflag {
		Intention, err = JudgeIntention(c, resptext)
		if err != nil {
			return err
		}
	}
	if (Intention == "想和对方聊天" || Intention == "想和对方倾诉") && tmpflag {
		global.LongChainflag = true
		err := ChatwithAi(c, resptext)
		if err != nil {
			return fmt.Errorf("ChatwithAi error in ResponseUserMsg: %v", err)
		}
		return nil
	}

	err = service.SendMsg(c, config.Config.TargetId, sendmsg)
	if err != nil {
		return err
	}
	return nil
}

func JudgeEmotion(c *websocket.Conn, resptext string) (result string, err error) {
	prompt := "请帮我分析下这段话的情感，并在下面六个选项中选择：开心，生气，中性，哲学，敷衍，难过， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容，也不需要添加分号"
	resp, err := aifunction.Queryai(prompt, resptext)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func JudgeIntention(c *websocket.Conn, resptext string) (result string, err error) {
	prompt := "请帮我分析下这段话的意图，并在下面六个选项中选择：想和对方聊天，想被对方鼓励，想和对方倾诉，安慰对方，鼓励对方，和对方道歉 并只回复选项，例如：\"user: 能陪我会儿吗\" resp: \"想和对方倾诉\", 不需要回答多余的内容，也不需要添加分号"
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
		log.Print("长对话ai聊天结束")
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
