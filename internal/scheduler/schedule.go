package scheduler

import (
	"time"

	"project-yume/internal/state"
)

func (ns *NaturalScheduler) RescheduleFrom(sessionID string, baseTime time.Time) time.Time {
	if baseTime.IsZero() {
		baseTime = time.Now()
	}
	next := baseTime.Add(ns.GetNextIntervalForSession(sessionID))
	state.GetManager().SetNextScheduledAt(sessionID, next)
	return next
}

func (ns *NaturalScheduler) EnsureScheduled(sessionID string, now time.Time) time.Time {
	next := state.GetManager().GetNextScheduledAt(sessionID)
	if !next.IsZero() {
		return next
	}

	baseTime := state.GetManager().GetLastInteractionAt(sessionID)
	if baseTime.IsZero() {
		baseTime = now
	}
	return ns.RescheduleFrom(sessionID, baseTime)
}

func (ns *NaturalScheduler) GetNextIntervalForSession(sessionID string) time.Duration {
	sm := state.GetManager()
	interval := ns.GetNextInterval()

	switch sm.GetState(sessionID) {
	case state.StateLongChat:
		return time.Duration(float64(interval) * 1.6)
	case state.StateBusy:
		return time.Duration(float64(interval) * 1.3)
	default:
		return interval
	}
}

func (ns *NaturalScheduler) ShouldSendNow(sessionID string, now time.Time) (bool, time.Time) {
	next := ns.EnsureScheduled(sessionID, now)
	return !now.Before(next), next
}

func (ns *NaturalScheduler) SweepInterval() time.Duration {
	return defaultSchedulerSweepInterval
}
