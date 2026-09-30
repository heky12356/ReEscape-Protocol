package intent

import "time"

type Kind string

const (
	KindProactiveContact Kind = "proactive_contact"
	KindFollowUp         Kind = "follow_up"
	KindDeliveryRetry    Kind = "delivery_retry"
	KindCheckOpenLoop    Kind = "check_open_loop"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusClaimed   Status = "claimed"
	StatusCompleted Status = "completed"
	StatusDeferred  Status = "deferred"
	StatusCancelled Status = "cancelled"
)

type Intent struct {
	ID           string         `json:"id"`
	UserID       int64          `json:"user_id"`
	SessionID    string         `json:"session_id"`
	Kind         Kind           `json:"kind"`
	Summary      string         `json:"summary"`
	Action       string         `json:"action,omitempty"`
	DueAt        time.Time      `json:"due_at"`
	Status       Status         `json:"status"`
	Priority     int            `json:"priority,omitempty"`
	Source       string         `json:"source,omitempty"`
	OpenLoopID   string         `json:"open_loop_id,omitempty"`
	SourceTurnID string         `json:"source_turn_id,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  time.Time      `json:"completed_at,omitempty"`
	CancelledAt  time.Time      `json:"cancelled_at,omitempty"`
}
