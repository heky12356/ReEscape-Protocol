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

const DefaultMaxRetries = 3

const (
	StatusPending   Status = "pending"
	StatusClaimed   Status = "claimed"
	StatusCompleted Status = "completed"
	StatusDeferred  Status = "deferred"
	StatusCancelled Status = "cancelled"
	// StatusPaused means automatic execution stopped after retry exhaustion.
	// It remains a terminal scheduler state until an explicit user action
	// moves the intent back to pending.
	StatusPaused Status = "paused"
)

type Intent struct {
	ID                 string         `json:"id"`
	UserID             int64          `json:"user_id"`
	SessionID          string         `json:"session_id"`
	Kind               Kind           `json:"kind"`
	Summary            string         `json:"summary"`
	Action             string         `json:"action,omitempty"`
	DueAt              time.Time      `json:"due_at"`
	Status             Status         `json:"status"`
	Priority           int            `json:"priority,omitempty"`
	Source             string         `json:"source,omitempty"`
	OpenLoopID         string         `json:"open_loop_id,omitempty"`
	SourceTurnID       string         `json:"source_turn_id,omitempty"`
	Meta               map[string]any `json:"meta,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CompletedAt        time.Time      `json:"completed_at,omitempty"`
	CancelledAt        time.Time      `json:"cancelled_at,omitempty"`
	RetryCount         int            `json:"retry_count,omitempty"`
	MaxRetries         int            `json:"max_retries,omitempty"`
	LastDeliveryStatus string         `json:"last_delivery_status,omitempty"`
	LastDeliveryError  string         `json:"last_delivery_error,omitempty"`
	LastDeliveryAt     time.Time      `json:"last_delivery_at,omitempty"`
}

func (item Intent) EffectiveMaxRetries() int {
	if item.MaxRetries <= 0 {
		return DefaultMaxRetries
	}
	return item.MaxRetries
}
