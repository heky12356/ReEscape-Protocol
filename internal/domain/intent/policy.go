package intent

import (
	"fmt"
	"strings"
	"time"
)

func Normalize(item Intent) Intent {
	item.ID = strings.TrimSpace(item.ID)
	item.SessionID = strings.TrimSpace(item.SessionID)
	item.Summary = strings.TrimSpace(item.Summary)
	item.Action = strings.TrimSpace(item.Action)
	item.Source = strings.TrimSpace(item.Source)
	item.OpenLoopID = strings.TrimSpace(item.OpenLoopID)
	item.SourceTurnID = strings.TrimSpace(item.SourceTurnID)
	if item.Status == "" {
		item.Status = StatusPending
	}
	if item.Action == "" {
		item.Action = string(item.Kind)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = item.UpdatedAt
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now()
	}
	return item
}

func IsDue(item Intent, now time.Time) bool {
	item = Normalize(item)
	return (item.Status == StatusPending || item.Status == StatusDeferred) && !item.DueAt.IsZero() && !now.Before(item.DueAt)
}

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusPending:
		return to == StatusClaimed || to == StatusCompleted || to == StatusDeferred || to == StatusCancelled || to == StatusPaused
	case StatusClaimed:
		return to == StatusPending || to == StatusCompleted || to == StatusDeferred || to == StatusCancelled || to == StatusPaused
	case StatusDeferred:
		return to == StatusPending || to == StatusClaimed || to == StatusCompleted || to == StatusCancelled || to == StatusPaused
	case StatusPaused:
		return to == StatusPending || to == StatusDeferred || to == StatusCompleted || to == StatusCancelled
	default:
		return false
	}
}

func Transition(item Intent, to Status, now time.Time) (Intent, error) {
	item = Normalize(item)
	if !CanTransition(item.Status, to) {
		return item, fmt.Errorf("invalid intent transition %s -> %s", item.Status, to)
	}
	if now.IsZero() {
		now = time.Now()
	}
	item.Status = to
	item.UpdatedAt = now
	if to == StatusCompleted {
		item.CompletedAt = now
	} else if to == StatusCancelled {
		item.CancelledAt = now
	}
	return item, nil
}
