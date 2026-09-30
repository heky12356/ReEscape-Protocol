package intent

import "time"

func Normalize(item Intent) Intent {
	if item.Status == "" {
		item.Status = StatusPending
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now()
	}
	return item
}

func IsDue(item Intent, now time.Time) bool {
	item = Normalize(item)
	return (item.Status == StatusPending || item.Status == StatusActive) && !item.DueAt.IsZero() && !now.Before(item.DueAt)
}
