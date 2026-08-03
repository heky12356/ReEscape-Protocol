package state

import (
	"testing"
	"time"
)

func TestStateManagerProactiveClaimAllowsOnlyOneActiveClaim(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))

	claimed, nextAt := sm.TryClaimProactiveTurn(sessionID, now, time.Minute)
	if !claimed {
		t.Fatalf("expected first claim to succeed, next=%s", nextAt)
	}
	if !sm.HasActiveProactiveClaim(sessionID, now.Add(30*time.Second)) {
		t.Fatalf("expected active proactive claim")
	}

	claimed, _ = sm.TryClaimProactiveTurn(sessionID, now.Add(30*time.Second), time.Minute)
	if claimed {
		t.Fatalf("expected second claim within lease to fail")
	}
}

func TestStateManagerProactiveClaimBlockedByPendingUserTurn(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))
	sm.MarkPendingUserTurn(sessionID, now)

	claimed, _ := sm.TryClaimProactiveTurn(sessionID, now, time.Minute)
	if claimed {
		t.Fatalf("expected pending user turn to block proactive claim")
	}

	sm.EndUserTurn(sessionID, now.Add(time.Second))
	claimed, _ = sm.TryClaimProactiveTurn(sessionID, now.Add(2*time.Second), time.Minute)
	if !claimed {
		t.Fatalf("expected claim after user turn ended")
	}
}

func TestStateManagerProactiveClaimLeaseExpiryAllowsReclaim(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.SetNextScheduledAt(sessionID, now.Add(-time.Minute))

	claimed, _ := sm.TryClaimProactiveTurn(sessionID, now, time.Second)
	if !claimed {
		t.Fatalf("expected initial claim")
	}

	claimed, _ = sm.TryClaimProactiveTurn(sessionID, now.Add(2*time.Second), time.Second)
	if !claimed {
		t.Fatalf("expected reclaim after lease expiry")
	}
}

func TestStateManagerEndUserTurnClearsPendingAndInFlight(t *testing.T) {
	sm := GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)

	sessionID := "private:42"
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	sm.MarkPendingUserTurn(sessionID, now)
	sm.BeginUserTurn(sessionID, now.Add(time.Second))

	if !sm.HasPendingOrRunningUserTurn(sessionID) {
		t.Fatalf("expected running user turn")
	}

	sm.EndUserTurn(sessionID, now.Add(2*time.Second))
	if sm.HasPendingOrRunningUserTurn(sessionID) {
		t.Fatalf("expected user turn gate to be clear")
	}
}
