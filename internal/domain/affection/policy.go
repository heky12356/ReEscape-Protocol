package affection

import (
	"time"
)

type Policy struct {
	SingleTurnCap int
	DailyCap      int
	MinConfidence float64
}

func DefaultPolicy() Policy {
	return Policy{
		SingleTurnCap: 2,
		DailyCap:      5,
		MinConfidence: 0.6,
	}
}

func (p Policy) Apply(state State, req UpdateRequest, now time.Time) (UpdateResult, error) {
	if now.IsZero() {
		now = time.Now()
	}
	if p.SingleTurnCap <= 0 {
		p.SingleTurnCap = 2
	}
	if p.DailyCap <= 0 {
		p.DailyCap = 5
	}
	if p.MinConfidence <= 0 {
		p.MinConfidence = 0.6
	}

	if req.Confidence > 0 && req.Confidence < p.MinConfidence {
		return UpdateResult{
			State:          state,
			RequestedDelta: req.Delta,
			AppliedDelta:   0,
			Policy:         "low_confidence_blocked",
		}, nil
	}

	applied := clamp(req.Delta, -p.SingleTurnCap, p.SingleTurnCap)
	policy := "single_turn_cap"

	dateKey := now.Format("2006-01-02")
	daily := 0
	if state.DailyDelta != nil {
		daily = state.DailyDelta[dateKey]
	}
	minDailyDelta := -p.DailyCap - daily
	maxDailyDelta := p.DailyCap - daily
	if minDailyDelta > maxDailyDelta {
		applied = 0
		policy = "daily_cap"
	} else if applied > maxDailyDelta {
		applied = maxDailyDelta
		policy = "daily_cap"
	} else if applied < minDailyDelta {
		applied = minDailyDelta
		policy = "daily_cap"
	}

	newState := state
	newState.Score += applied
	newState.Stage = deriveStage(newState.Score)
	newState.LastReason = req.Reason
	newState.LastUpdated = now
	if newState.DailyDelta == nil {
		newState.DailyDelta = map[string]int{}
	}
	newState.DailyDelta[dateKey] = daily + applied
	newState.Changes = append(newState.Changes, ChangeRecord{
		RequestedDelta: req.Delta,
		AppliedDelta:   applied,
		Reason:         req.Reason,
		Confidence:     req.Confidence,
		Tags:           append([]string(nil), req.Tags...),
		Policy:         policy,
		CreatedAt:      now,
	})

	return UpdateResult{
		State:          newState,
		RequestedDelta: req.Delta,
		AppliedDelta:   applied,
		Policy:         policy,
	}, nil
}

func deriveStage(score int) string {
	switch {
	case score >= 60:
		return "affectionate"
	case score >= 20:
		return "warm"
	case score >= -20:
		return "neutral"
	case score >= -60:
		return "distant"
	default:
		return "guarded"
	}
}

func clamp(v, minV, maxV int) int {
	if minV > maxV {
		minV, maxV = maxV, minV
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
