package scheduler

import (
	"math/rand"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

type nowState int

const (
	isLongChat nowState = iota
	isBusy
	suitTime = 0 // 合适的时间
)

// NaturalScheduler 自然定时器
type NaturalScheduler struct {
	baseInterval    time.Duration
	randomFactor    float64
	activeHours     []int // 活跃时间段
	sleepHours      []int // 休息时间段
	messagePool     *MessagePool
	lastMessageTime time.Time
}

// MessagePool 消息池
type MessagePool struct {
	casual    []string       // 日常消息
	emotional []string       // 情感消息
	question  []string       // 问候消息
	weights   map[string]int // 消息类型权重
}

func NewNaturalScheduler() *NaturalScheduler {
	return &NaturalScheduler{
		baseInterval:    45 * time.Minute,                         // 基础间隔45分钟
		randomFactor:    0.5,                                      // 随机因子50%
		activeHours:     []int{9, 10, 11, 14, 15, 16, 19, 20, 21}, // 活跃时间
		sleepHours:      []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 22, 23}, // 休息时间
		messagePool:     newMessagePool(),
		lastMessageTime: time.Now(),
	}
}

func newMessagePool() *MessagePool {
	return &MessagePool{
		casual: []string{
			"在干嘛呢",
			"最近怎么样",
			"今天过得好吗",
			"有什么新鲜事吗",
			"忙什么呢",
		},
		emotional: []string{
			"想你了",
			"有点想聊天",
			"感觉有点无聊",
			"今天心情不错",
		},
		question: []string{
			"在吗",
			"睡了吗",
			"吃饭了吗",
			"休息了吗",
		},
		weights: map[string]int{
			"casual":    60, // 60%概率发日常消息
			"emotional": 25, // 25%概率发情感消息
			"question":  15, // 15%概率发问候消息
		},
	}
}

// GetNextInterval 计算下一次发送间隔
func (ns *NaturalScheduler) GetNextInterval() time.Duration {
	now := time.Now()
	hour := now.Hour()

	// 基础间隔调整
	var interval time.Duration
	if ns.isActiveHour(hour) {
		// 活跃时间：30-60分钟
		interval = time.Duration(30+rand.Intn(30)) * time.Minute
	} else if ns.isSleepHour(hour) {
		// 休息时间：2-4小时
		interval = time.Duration(2+rand.Intn(2)) * time.Hour
	} else {
		// 普通时间：45-90分钟
		interval = time.Duration(45+rand.Intn(45)) * time.Minute
	}

	// 添加随机因子
	randomOffset := time.Duration(float64(interval) * ns.randomFactor * (rand.Float64() - 0.5))
	interval += randomOffset

	return interval
}

// SelectMessage 智能选择消息
func (ns *NaturalScheduler) SelectMessage() string {
	now := time.Now()
	hour := now.Hour()

	// 根据时间调整消息类型权重
	weights := make(map[string]int)
	for k, v := range ns.messagePool.weights {
		weights[k] = v
	}

	// 晚上增加情感消息权重
	if hour >= 20 || hour <= 2 {
		weights["emotional"] += 20
		weights["casual"] -= 10
	}

	// 早上增加问候消息权重
	if hour >= 7 && hour <= 10 {
		weights["question"] += 15
		weights["casual"] -= 10
	}

	// 根据上次发送时间调整
	timeSinceLastMessage := time.Since(ns.lastMessageTime)
	if timeSinceLastMessage > 2*time.Hour {
		weights["question"] += 10 // 长时间未联系，增加问候
	}

	// 加权随机选择
	messageType := ns.weightedRandomSelect(weights)
	return ns.selectFromPool(messageType)
}

func (ns *NaturalScheduler) weightedRandomSelect(weights map[string]int) string {
	total := 0
	for _, weight := range weights {
		total += weight
	}

	r := rand.Intn(total)
	current := 0

	for msgType, weight := range weights {
		current += weight
		if r < current {
			return msgType
		}
	}

	return "casual" // 默认返回日常消息
}

func (ns *NaturalScheduler) selectFromPool(messageType string) string {
	var pool []string

	switch messageType {
	case "casual":
		pool = ns.messagePool.casual
	case "emotional":
		pool = ns.messagePool.emotional
	case "question":
		pool = ns.messagePool.question
	default:
		pool = ns.messagePool.casual
	}

	if len(pool) == 0 {
		return "在干嘛"
	}

	return pool[rand.Intn(len(pool))]
}

func (ns *NaturalScheduler) isActiveHour(hour int) bool {
	for _, h := range ns.activeHours {
		if h == hour {
			return true
		}
	}
	return false
}

func (ns *NaturalScheduler) isSleepHour(hour int) bool {
	for _, h := range ns.sleepHours {
		if h == hour {
			return true
		}
	}
	return false
}

// ShouldSend 判断是否应该发送消息
func (ns *NaturalScheduler) ShouldSend() nowState {
	sm := state.GetManager()

	// 如果正在长对话，不发送
	if sm.GetState() == state.StateLongChat {
		return isLongChat
	}

	// 如果最近刚回复过，延长间隔
	if sm.GetState() == state.StateBusy && sm.GetTimeSinceLastReply() < 1*time.Hour {
		return isBusy
	}

	return suitTime
}

// SendScheduledMessage 发送定时消息
func (ns *NaturalScheduler) SendScheduledMessage(c *websocket.Conn) error {
	// 看下是在长对话还是最近刚发过消息
	nowstate := ns.ShouldSend()
	if nowstate != suitTime {
		utils.Info("当前状态不适合发送消息, state: %v", nowstate)
		return nil
	}

	message := ns.SelectMessage()
	ns.lastMessageTime = time.Now()

	// 根据消息类型设置状态
	if message == "想你了" || message == "有点想聊天" {
		state.GetManager().SetState(state.StateNeedComfort)
	}

	return service.SendMsg(c, config.GetConfig().TargetId, message)
}
