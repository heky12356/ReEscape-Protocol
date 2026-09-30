package openloop

import "time"

type Kind string

const (
	KindUnansweredQuestion Kind = "unanswered_question"
	KindUnresolvedProblem  Kind = "unresolved_problem"
	KindFollowUp           Kind = "assistant_follow_up"
	KindPendingThread      Kind = "pending_user_thread"
)

type Status string

const (
	StatusOpen      Status = "open"
	StatusActive    Status = "active"
	StatusResolved  Status = "resolved"
	StatusDeferred  Status = "deferred"
	StatusCancelled Status = "cancelled"
)

type OpenLoop struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	SessionID    string    `json:"session_id"`
	Kind         Kind      `json:"kind"`
	Description  string    `json:"description"`
	Source       string    `json:"source,omitempty"`
	SourceTurnID string    `json:"source_turn_id,omitempty"`
	TurnIndex    int       `json:"turn_index,omitempty"`
	IntentID     string    `json:"intent_id,omitempty"`
	Status       Status    `json:"status"`
	Priority     int       `json:"priority,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DueAt        time.Time `json:"due_at,omitempty"`
	ClosedAt     time.Time `json:"closed_at,omitempty"`
	ClosedBy     string    `json:"closed_by,omitempty"`
}
