package intent

import (
	"testing"
	"time"
)

func TestIntentDueOnlyWhilePendingOrActive(t *testing.T) {
	now := time.Now()
	item := Intent{DueAt: now.Add(-time.Second)}
	if !IsDue(item, now) {
		t.Fatal("expected pending intent to be due")
	}
	item.Status = StatusCompleted
	if IsDue(item, now) {
		t.Fatal("completed intent must not be due")
	}
}
