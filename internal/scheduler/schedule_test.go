package scheduler

import (
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/state"
)

func TestNaturalSchedulerTryClaimDueClaimsOnlyOnce(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	cfg := config.GetConfig()
	oldLease := cfg.ProactiveClaimLeaseMs
	oldGrace := cfg.ProactiveUserMessageGraceMs
	t.Cleanup(func() {
		cfg.ProactiveClaimLeaseMs = oldLease
		cfg.ProactiveUserMessageGraceMs = oldGrace
	})
	cfg.ProactiveClaimLeaseMs = 60000
	cfg.ProactiveUserMessageGraceMs = 1

	ns := NewNaturalScheduler()
	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))

	claimed, _ := ns.TryClaimDue(sessionID, now)
	if !claimed {
		t.Fatalf("expected due schedule to be claimed")
	}
	claimed, _ = ns.TryClaimDue(sessionID, now.Add(time.Second))
	if claimed {
		t.Fatalf("expected active claim to block duplicate claim")
	}
}

func TestNaturalSchedulerTryClaimDueBlockedByPendingUserTurn(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	cfg := config.GetConfig()
	oldSkip := cfg.ProactiveSkipOnPendingUser
	oldGrace := cfg.ProactiveUserMessageGraceMs
	t.Cleanup(func() {
		cfg.ProactiveSkipOnPendingUser = oldSkip
		cfg.ProactiveUserMessageGraceMs = oldGrace
	})
	cfg.ProactiveSkipOnPendingUser = true
	cfg.ProactiveUserMessageGraceMs = 1

	ns := NewNaturalScheduler()
	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))
	sm.MarkPendingUserTurn(sessionID, now)

	claimed, _ := ns.TryClaimDue(sessionID, now)
	if claimed {
		t.Fatalf("expected pending user turn to block scheduler claim")
	}
}

func TestNaturalSchedulerTryClaimDueAllowsReclaimAfterLeaseExpiry(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	cfg := config.GetConfig()
	oldLease := cfg.ProactiveClaimLeaseMs
	oldGrace := cfg.ProactiveUserMessageGraceMs
	t.Cleanup(func() {
		cfg.ProactiveClaimLeaseMs = oldLease
		cfg.ProactiveUserMessageGraceMs = oldGrace
	})
	cfg.ProactiveClaimLeaseMs = 1000
	cfg.ProactiveUserMessageGraceMs = 1

	ns := NewNaturalScheduler()
	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))

	claimed, _ := ns.TryClaimDue(sessionID, now)
	if !claimed {
		t.Fatalf("expected initial claim")
	}
	claimed, _ = ns.TryClaimDue(sessionID, now.Add(2*time.Second))
	if !claimed {
		t.Fatalf("expected claim after lease expiry")
	}
}
