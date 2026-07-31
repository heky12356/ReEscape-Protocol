package affection

import (
	"testing"
	"time"
)

func TestPolicyCapsSingleTurnDelta(t *testing.T) {
	policy := DefaultPolicy()
	result, err := policy.Apply(State{UserID: 1, DailyDelta: map[string]int{}}, UpdateRequest{
		Delta:      8,
		Reason:     "positive signal",
		Confidence: 0.8,
	}, time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedDelta != 2 {
		t.Fatalf("expected applied delta 2, got %d", result.AppliedDelta)
	}
}

func TestPolicyCapsDailyPositiveAndNegativeBounds(t *testing.T) {
	policy := DefaultPolicy()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	dateKey := now.Format("2006-01-02")

	positive, err := policy.Apply(State{
		UserID:     1,
		DailyDelta: map[string]int{dateKey: 4},
	}, UpdateRequest{
		Delta:      2,
		Reason:     "positive signal",
		Confidence: 0.8,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if positive.AppliedDelta != 1 {
		t.Fatalf("expected positive daily cap to apply 1, got %d", positive.AppliedDelta)
	}

	negative, err := policy.Apply(State{
		UserID:     1,
		DailyDelta: map[string]int{dateKey: -4},
	}, UpdateRequest{
		Delta:      -2,
		Reason:     "negative signal",
		Confidence: 0.8,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if negative.AppliedDelta != -1 {
		t.Fatalf("expected negative daily cap to apply -1, got %d", negative.AppliedDelta)
	}
}

func TestPolicyBlocksLowConfidence(t *testing.T) {
	policy := DefaultPolicy()
	result, err := policy.Apply(State{UserID: 1, DailyDelta: map[string]int{}}, UpdateRequest{
		Delta:      1,
		Reason:     "weak signal",
		Confidence: 0.2,
	}, time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedDelta != 0 || result.Policy != "low_confidence_blocked" {
		t.Fatalf("expected low confidence block, got delta=%d policy=%s", result.AppliedDelta, result.Policy)
	}
}
