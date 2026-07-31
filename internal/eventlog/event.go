package eventlog

import "time"

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	SessionID string         `json:"session_id,omitempty"`
	UserID    int64          `json:"user_id,omitempty"`
	Actor     string         `json:"actor,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func New(eventType string) Event {
	return Event{
		Type:      eventType,
		CreatedAt: time.Now(),
	}
}
