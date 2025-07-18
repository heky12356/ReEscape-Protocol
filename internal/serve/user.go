package serve

import (
	"fmt"
	"log"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/memory"
	"project-yume/internal/service"
	"project-yume/internal/state"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

// LegacyResponseUserMsg 保持向后兼容的消息处理函数
func LegacyResponseUserMsg(c *websocket.Conn, resptext string) error {
	sm := state.GetManager()

	// 检查是否在长对话模式
	if sm.GetFlag(state.FlagLongChain) {
		return ChatwithAi(c, resptext)
	}

	var emotion, intention string
	var err error

	// 处理敷衍状态
	if sm.GetFlag(state.FlagPerfunctory) {
		intention, err = JudgeIntention(c, resptext)
		if err != nil {
			return err
		}
		emotion, err = JudgeEmotion(c, resptext)
		if err != nil {
			return err
		}

		if emotion == "生气" {
			err := service.SendMsg(c, config.GetConfig().TargetId, "别生气")
			if err != nil {
				return fmt.Errorf("SendMsg error: %v", err)
			}
			sm.SetFlag(state.FlagPerfunctory, false)
			return nil
		}

		if intention == "和对方道歉" {
			err := service.SendMsg(c, config.GetConfig().TargetId, "没事，我没生气")
			if err != nil {
				return fmt.Errorf("SendMsg error: %v", err)
			}
			err = service.SendMsg(c, config.GetConfig().TargetId, "真的")
			if err != nil {
				return fmt.Errorf("SendMsg error: %v", err)
			}
			sm.SetFlag(state.FlagPerfunctory, false)
			return nil
		}
	}

	// 处理需要鼓励状态
	if sm.GetFlag(state.FlagNeedEncourage) {
		if !sm.GetFlag(state.FlagEncourage) {
			intention, err = JudgeIntention(c, resptext)
			if err != nil {
				return err
			}
			emotion, err = JudgeEmotion(c, resptext)
			if err != nil {
				return err
			}

			if emotion == "敷衍" {
				err := service.SendMsg(c, config.GetConfig().TargetId, "敷衍")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				sm.SetFlag(state.FlagPerfunctory, true)
				return nil
			}

			if intention == "鼓励对方" {
				err := service.SendMsg(c, config.GetConfig().TargetId, "嘿嘿")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				err = service.SendMsg(c, config.GetConfig().TargetId, "谢谢")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				sm.SetFlag(state.FlagEncourage, true)
				return nil
			} else {
				err := service.SendMsg(c, config.GetConfig().TargetId, "快鼓励我！！")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				return nil
			}
		} else {
			sm.SetFlag(state.FlagNeedEncourage, false)
		}
	}

	// 处理需要安慰状态
	if sm.GetFlag(state.FlagNeedComfort) {
		if !sm.GetFlag(state.FlagComfort) {
			intention, err = JudgeIntention(c, resptext)
			if err != nil {
				return err
			}
			emotion, err = JudgeEmotion(c, resptext)
			if err != nil {
				return err
			}

			if emotion == "敷衍" {
				err := service.SendMsg(c, config.GetConfig().TargetId, "敷衍")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				sm.SetFlag(state.FlagPerfunctory, true)
				return nil
			}

			if intention == "安慰对方" {
				err := service.SendMsg(c, config.GetConfig().TargetId, "嘿嘿")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				err = service.SendMsg(c, config.GetConfig().TargetId, "谢谢")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				sm.SetFlag(state.FlagComfort, true)
				return nil
			} else {
				err := service.SendMsg(c, config.GetConfig().TargetId, "sad")
				if err != nil {
					return fmt.Errorf("SendMsg error: %v", err)
				}
				return nil
			}
		} else {
			sm.SetFlag(state.FlagNeedComfort, false)
		}
	}

	// 预设回复处理
	sendmsg := getPresetResponse(resptext)

	if sendmsg == "" {
		// 如果没有预设回复，进行情感分析
		emotion, err = JudgeEmotion(c, resptext)
		if err != nil {
			return err
		}

		sendmsg = getEmotionResponse(emotion)

		// 如果仍然没有合适回复，进行意图分析
		if sendmsg == "?" {
			intention, err = JudgeIntention(c, resptext)
			if err != nil {
				return err
			}

			if intention == "想和对方聊天" || intention == "想和对方倾诉" {
				sm.SetFlag(state.FlagLongChain, true)
				return ChatwithAi(c, resptext)
			}
		}
	}

	// 发送回复
	err = service.SendMsg(c, config.GetConfig().TargetId, sendmsg)
	if err != nil {
		return err
	}

	// 更新状态
	sm.UpdateLastReply()

	// 记录到情感记忆（如果启用）
	if config.GetConfig().EnableEmotionalMemory {
		memoryManager := memory.GetManager()
		memoryManager.RecordInteraction(config.GetConfig().TargetId, resptext, sendmsg, emotion, intention)
	}

	return nil
}

// getPresetResponse 获取预设回复
func getPresetResponse(text string) string {
	sm := state.GetManager()

	switch text {
	case "你好":
		return "你好"
	case "在干嘛":
		return "在学习"
	case "在忙呢":
		sm.SetFlag(state.FlagBusy, true)
		return "好吧"
	case "难过了":
		return "别难过，开心点，加油！"
	case "我不信":
		return "[CQ:image,type=image,url=https://pan.heky.top/photo/v2-58816628de7a7812f1afd46fd411090c_b.jpg,title=image]"
	case "我想你了":
		return "是嘛？嘿嘿"
	case "能陪我聊聊吗":
		sm.SetFlag(state.FlagLongChain, true)
		sm.AddToConversation(openai.ChatCompletionMessage{Role: "user", Content: "能陪我聊聊吗"})
		sm.AddToConversation(openai.ChatCompletionMessage{Role: "assistant", Content: "好"})
		return "好"
	default:
		return ""
	}
}

// getEmotionResponse 根据情感获取回复
func getEmotionResponse(emotion string) string {
	switch emotion {
	case "开心":
		return "那很好了。"
	case "生气":
		return "我做错什么了？"
	case "哲学":
		return "乐"
	default:
		return "?"
	}
}

// JudgeEmotion 情感分析
func JudgeEmotion(c *websocket.Conn, resptext string) (string, error) {
	prompt := "请帮我分析下这段话的情感，并在下面六个选项中选择：开心，生气，中性，哲学，敷衍，难过， 并只回复选项，例如：\"user: 哈哈哈\" resp: \"开心\", 不需要回答多余的内容，也不需要添加分号"
	resp, err := aifunction.Queryai(prompt, resptext)
	if err != nil {
		return "", fmt.Errorf("Queryai error in JudgeEmotion: %v", err)
	}
	return resp, nil
}

// JudgeIntention 意图分析
func JudgeIntention(c *websocket.Conn, resptext string) (string, error) {
	prompt := "请帮我分析下这段话的意图，并在下面六个选项中选择：想和对方聊天，想被对方鼓励，想和对方倾诉，安慰对方，鼓励对方，和对方道歉 并只回复选项，例如：\"user: 能陪我会儿吗\" resp: \"想和对方倾诉\", 不需要回答多余的内容，也不需要添加分号"
	resp, err := aifunction.Queryai(prompt, resptext)
	if err != nil {
		return "", fmt.Errorf("Queryai error in JudgeIntention: %v", err)
	}
	return resp, nil
}

// ChatwithAi AI长对话处理
func ChatwithAi(c *websocket.Conn, msg string) error {
	sm := state.GetManager()

	if msg == "不聊了" {
		err := service.SendMsg(c, config.GetConfig().TargetId, "好吧")
		if err != nil {
			return fmt.Errorf("SendMsg error in ChatwithAi: %v", err)
		}
		sm.SetFlag(state.FlagLongChain, false)
		sm.ClearConversation()
		log.Print("长对话ai聊天结束")
		return nil
	}

	filepath := "./public/aichatlog/longchain/log_" + time.Now().Format("06-01-02") + ".txt"
	conversation := sm.GetConversation()
	conversation = append(conversation, openai.ChatCompletionMessage{Role: "user", Content: msg})

	newConversation, result, err := aifunction.QueryaiWithChain(conversation, filepath)
	if err != nil {
		return err
	}

	for _, res := range result {
		err := service.SendMsg(c, config.GetConfig().TargetId, res)
		if err != nil {
			return fmt.Errorf("SendMsg error in ChatwithAi: %v", err)
		}
	}

	sm.SetConversation(newConversation)
	return nil
}

// ResponseUserMsg 新的消息处理函数，使用新架构
func ResponseUserMsg(c *websocket.Conn, resptext string) error {
	// 为了保持向后兼容，暂时调用旧的实现
	// 后续可以逐步迁移到新的处理器架构
	return LegacyResponseUserMsg(c, resptext)
}
