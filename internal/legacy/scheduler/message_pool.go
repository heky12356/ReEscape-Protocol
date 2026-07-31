package scheduler

import (
	"math/rand"
	"time"

	corescheduler "project-yume/internal/scheduler"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

type MessagePool struct {
	casual    []string
	emotional []string
	question  []string
	weights   map[string]int
}

var defaultMessagePool = newMessagePool()

func SendScheduledMessage(c *websocket.Conn, scheduler *corescheduler.NaturalScheduler, sessionID string, targetUserID int64) error {
	state.GetManager().EnsureSession(sessionID, targetUserID, 0, 1)
	if shouldSend, nextAt := scheduler.ShouldSendNow(sessionID, time.Now()); !shouldSend {
		utils.Info("主动消息发送前检查未到时间, next=%s", nextAt.Format(time.RFC3339))
		return nil
	}
	message := SelectMessage(sessionID)

	if message == "想你了" || message == "有点想聊天" {
		state.GetManager().SetState(sessionID, state.StateNeedComfort)
	}

	if err := service.SendMsg(c, targetUserID, message); err != nil {
		return err
	}

	sentAt := time.Now()
	state.GetManager().RecordAssistantTurn(sessionID, service.BuildAssistantTranscript(message), sentAt, true)
	state.GetManager().UpdateLastReplyMode(sessionID, "proactive")
	next := scheduler.RescheduleFrom(sessionID, sentAt)
	utils.Info("主动消息已发送，下一次主动触达时间: %s", next.Format(time.RFC3339))
	return nil
}

func SelectMessage(sessionID string) string {
	now := time.Now()
	hour := now.Hour()

	weights := make(map[string]int)
	for k, v := range defaultMessagePool.weights {
		weights[k] = v
	}

	if hour >= 20 || hour <= 2 {
		weights["emotional"] += 20
		weights["casual"] -= 10
	}
	if hour >= 7 && hour <= 10 {
		weights["question"] += 15
		weights["casual"] -= 10
	}

	timeSinceLastInteraction := state.GetManager().GetTimeSinceLastInteraction(sessionID)
	if timeSinceLastInteraction > 2*time.Hour {
		weights["question"] += 10
	}

	messageType := weightedRandomSelect(weights)
	return selectFromPool(messageType)
}

func newMessagePool() *MessagePool {
	return &MessagePool{
		casual: []string{
			"在干嘛呢",
			"在忙什么呢",
		},
		emotional: []string{
			"想你了",
			"有点想聊天",
			"无聊了",
		},
		question: []string{
			"在吗在吗",
			"在干嘛呢",
		},
		weights: map[string]int{
			"casual":    60,
			"emotional": 25,
			"question":  15,
		},
	}
}

func weightedRandomSelect(weights map[string]int) string {
	total := 0
	for _, weight := range weights {
		if weight > 0 {
			total += weight
		}
	}
	if total <= 0 {
		return "casual"
	}

	r := rand.Intn(total)
	current := 0
	for msgType, weight := range weights {
		if weight <= 0 {
			continue
		}
		current += weight
		if r < current {
			return msgType
		}
	}
	return "casual"
}

func selectFromPool(messageType string) string {
	var pool []string

	switch messageType {
	case "casual":
		pool = defaultMessagePool.casual
	case "emotional":
		pool = defaultMessagePool.emotional
	case "question":
		pool = defaultMessagePool.question
	default:
		pool = defaultMessagePool.casual
	}

	if len(pool) == 0 {
		return "在干嘛"
	}

	return pool[rand.Intn(len(pool))]
}
