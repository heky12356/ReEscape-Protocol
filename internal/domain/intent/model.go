package intent

import "time"

type Kind string

const (
	KindProactiveContact Kind = "proactive_contact"
	KindFollowUp         Kind = "follow_up"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusDeferred  Status = "deferred"
	StatusCancelled Status = "cancelled"
)

type Intent struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id"`
	SessionID  string    `json:"session_id"`
	Kind       Kind      `json:"kind"`
	Summary    string    `json:"summary"`
	DueAt      time.Time `json:"due_at"`
	Status     Status    `json:"status"`
	Source     string    `json:"source,omitempty"`
	OpenLoopID string    `json:"open_loop_id,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}
