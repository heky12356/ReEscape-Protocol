package scheduler

import (
	"time"

	"project-yume/internal/config"
	"project-yume/internal/domain/intent"
	"project-yume/internal/state"
)

func (ns *NaturalScheduler) TryClaimIntentDue(userID int64, sessionID string, now time.Time) (intent.Intent, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	if !ns.IsIdleForProactive(sessionID, now) {
		return intent.Intent{}, false
	}
	claimed, ok := state.GetManager().IntentStore().ClaimDue(userID, sessionID, now)
	if !ok {
		return intent.Intent{}, false
	}
	proactiveClaimed := state.GetManager().TryClaimProactiveTurnForIntent(sessionID, now, ns.claimLease(), config.GetConfig().ProactiveSkipOnPendingUser, ns.userMessageGrace())
	if !proactiveClaimed {
		_, _ = state.GetManager().IntentStore().Transition(claimed.ID, intent.StatusPending, now)
		return intent.Intent{}, false
	}
	return claimed, true
}

func (ns *NaturalScheduler) ReleaseIntent(id string) {
	if id == "" {
		return
	}
	_, _ = state.GetManager().IntentStore().Transition(id, intent.StatusPending, time.Now())
}

func (ns *NaturalScheduler) CompleteIntent(id string, at time.Time) {
	if id == "" {
		return
	}
	_, _ = state.GetManager().IntentStore().Transition(id, intent.StatusCompleted, at)
}

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

func (ns *NaturalScheduler) TryClaimDue(sessionID string, now time.Time) (bool, time.Time) {
	next := ns.EnsureScheduled(sessionID, now)
	if now.Before(next) {
		return false, next
	}

	claimLease := ns.claimLease()
	userGrace := ns.userMessageGrace()
	cfg := config.GetConfig()
	return state.GetManager().TryClaimProactiveTurnWithUserGate(
		sessionID,
		now,
		claimLease,
		cfg.ProactiveSkipOnPendingUser,
		userGrace,
	)
}

func (ns *NaturalScheduler) ReleaseClaim(sessionID string) {
	state.GetManager().ReleaseProactiveClaim(sessionID)
}

func (ns *NaturalScheduler) CompleteClaim(sessionID string, sentAt time.Time) {
	state.GetManager().CompleteProactiveTurn(sessionID, sentAt)
}

func (ns *NaturalScheduler) IsIdleForProactive(sessionID string, now time.Time) bool {
	cfg := config.GetConfig()
	if !cfg.ProactiveSkipOnPendingUser {
		return true
	}
	return state.GetManager().IsSessionIdleForProactiveWithGrace(sessionID, now, ns.userMessageGrace())
}

func (ns *NaturalScheduler) SweepInterval() time.Duration {
	return defaultSchedulerSweepInterval
}

func (ns *NaturalScheduler) claimLease() time.Duration {
	lease := time.Duration(config.GetConfig().ProactiveClaimLeaseMs) * time.Millisecond
	if lease <= 0 {
		return 2 * time.Minute
	}
	return lease
}

func (ns *NaturalScheduler) userMessageGrace() time.Duration {
	cfg := config.GetConfig()
	grace := time.Duration(cfg.ProactiveUserMessageGraceMs) * time.Millisecond
	if grace <= 0 {
		grace = 30 * time.Second
	}

	aggregateMaxWindow := time.Duration(cfg.MessageAggregateMaxWindowMs) * time.Millisecond
	if aggregateMaxWindow > grace {
		return aggregateMaxWindow
	}
	return grace
}
