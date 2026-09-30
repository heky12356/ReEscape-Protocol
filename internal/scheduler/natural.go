package scheduler

import (
	"math/rand"
	"time"

	"project-yume/internal/config"
)

const defaultSchedulerSweepInterval = 15 * time.Second

// NaturalScheduler owns proactive timing only. ReAct owns proactive message selection and delivery.
type NaturalScheduler struct {
	baseInterval time.Duration
	randomFactor float64
	activeHours  []int
	sleepHours   []int
}

func NewNaturalScheduler() *NaturalScheduler {
	ns := &NaturalScheduler{}
	ns.reloadConfig()
	return ns
}

// GetNextInterval calculates the next proactive trigger interval.
func (ns *NaturalScheduler) GetNextInterval() time.Duration {
	ns.reloadConfig()

	now := time.Now()
	hour := now.Hour()
	baseMinutes := int(ns.baseInterval / time.Minute)
	if baseMinutes <= 0 {
		baseMinutes = 45
	}

	var interval time.Duration
	if ns.isActiveHour(hour) {
		interval = randomDurationInMinutes(maxInt(5, (baseMinutes*2)/3), maxInt(5, (baseMinutes*4)/3))
	} else if ns.isSleepHour(hour) {
		interval = randomDurationInMinutes(maxInt(120, baseMinutes*3), maxInt(120, baseMinutes*6))
	} else {
		interval = randomDurationInMinutes(maxInt(5, baseMinutes), maxInt(5, baseMinutes*2))
	}

	randomOffset := time.Duration(float64(interval) * ns.randomFactor * (rand.Float64() - 0.5))
	return interval + randomOffset
}

func (ns *NaturalScheduler) reloadConfig() {
	cfg := config.GetConfig()

	baseMinutes := cfg.BaseInterval
	if baseMinutes <= 0 {
		baseMinutes = 45
	}
	ns.baseInterval = time.Duration(baseMinutes) * time.Minute

	randomFactor := cfg.RandomFactor
	if randomFactor < 0 {
		randomFactor = 0
	}
	ns.randomFactor = randomFactor

	if len(cfg.ActiveHours) == 0 {
		ns.activeHours = []int{9, 10, 11, 14, 15, 16, 19, 20, 21}
	} else {
		ns.activeHours = append([]int(nil), cfg.ActiveHours...)
	}

	if len(cfg.SleepHours) == 0 {
		ns.sleepHours = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 22, 23}
	} else {
		ns.sleepHours = append([]int(nil), cfg.SleepHours...)
	}
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

func randomDurationInMinutes(minMinutes, maxMinutes int) time.Duration {
	if minMinutes <= 0 {
		minMinutes = 1
	}
	if maxMinutes < minMinutes {
		maxMinutes = minMinutes
	}
	if maxMinutes == minMinutes {
		return time.Duration(minMinutes) * time.Minute
	}
	return time.Duration(minMinutes+rand.Intn(maxMinutes-minMinutes+1)) * time.Minute
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
