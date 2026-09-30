package openloop

import (
	"fmt"
	"strings"
	"time"
)

func Normalize(loop OpenLoop) OpenLoop {
	loop.ID = strings.TrimSpace(loop.ID)
	loop.SessionID = strings.TrimSpace(loop.SessionID)
	loop.Description = strings.TrimSpace(loop.Description)
	loop.Source = strings.TrimSpace(loop.Source)
	loop.SourceTurnID = strings.TrimSpace(loop.SourceTurnID)
	loop.IntentID = strings.TrimSpace(loop.IntentID)
	loop.ClosedBy = strings.TrimSpace(loop.ClosedBy)
	loop.Status = Status(strings.TrimSpace(string(loop.Status)))
	if loop.Status == "" {
		loop.Status = StatusOpen
	}
	if loop.Source == "" {
		loop.Source = "heuristic"
	}
	if loop.Priority < 0 {
		loop.Priority = 0
	}
	if loop.UpdatedAt.IsZero() {
		loop.UpdatedAt = loop.CreatedAt
		if loop.UpdatedAt.IsZero() {
			loop.UpdatedAt = time.Now()
		}
	}
	return loop
}

func IsActionable(loop OpenLoop) bool {
	loop = Normalize(loop)
	return loop.Description != "" && (loop.Status == StatusOpen || loop.Status == StatusActive)
}

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusOpen:
		return to == StatusActive || to == StatusResolved || to == StatusDeferred || to == StatusCancelled
	case StatusActive:
		return to == StatusResolved || to == StatusDeferred || to == StatusCancelled || to == StatusOpen
	case StatusDeferred:
		return to == StatusOpen || to == StatusActive || to == StatusCancelled
	default:
		return false
	}
}

func Transition(loop OpenLoop, to Status, actor string, now time.Time) (OpenLoop, error) {
	loop = Normalize(loop)
	to = Status(strings.TrimSpace(string(to)))
	if to == "" {
		return loop, fmt.Errorf("open loop status is required")
	}
	if !CanTransition(loop.Status, to) {
		return loop, fmt.Errorf("invalid open loop transition %s -> %s", loop.Status, to)
	}
	if now.IsZero() {
		now = time.Now()
	}
	loop.Status = to
	loop.UpdatedAt = now
	if to == StatusResolved || to == StatusCancelled {
		loop.ClosedAt = now
		loop.ClosedBy = strings.TrimSpace(actor)
	} else {
		loop.ClosedAt = time.Time{}
		loop.ClosedBy = ""
	}
	return loop, nil
}

func IsDue(loop OpenLoop, now time.Time) bool {
	loop = Normalize(loop)
	if loop.DueAt.IsZero() || (loop.Status != StatusOpen && loop.Status != StatusActive) {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(loop.DueAt)
}
