package openloop

import "time"

type Kind string

const (
	KindUnansweredQuestion Kind = "unanswered_question"
	KindUnresolvedProblem  Kind = "unresolved_problem"
	KindFollowUp           Kind = "assistant_follow_up"
	KindPendingThread      Kind = "pending_user_thread"
)

type OpenLoop struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	SessionID   string    `json:"session_id"`
	Kind        Kind      `json:"kind"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	DueAt       time.Time `json:"due_at,omitempty"`
	Status      string    `json:"status"`
}
