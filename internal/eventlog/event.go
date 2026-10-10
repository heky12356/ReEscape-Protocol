package eventlog

import "time"

// Correlation contains the identifiers shared by a request, turn, tool call,
// intent, and delivery. Empty fields are intentionally omitted from JSON so
// older event snapshots remain compatible.
type Correlation struct {
	RequestID  string
	TurnID     string
	IntentID   string
	DeliveryID string
	SessionID  string
	UserID     int64
	Actor      string
	ToolName   string
	ToolCallID string
	Provider   string
	Model      string
}

type Event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	RequestID  string         `json:"request_id,omitempty"`
	TurnID     string         `json:"turn_id,omitempty"`
	IntentID   string         `json:"intent_id,omitempty"`
	DeliveryID string         `json:"delivery_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	UserID     int64          `json:"user_id,omitempty"`
	Actor      string         `json:"actor,omitempty"`
	Tool       string         `json:"tool,omitempty"` // Deprecated: use ToolName.
	ToolName   string         `json:"tool_name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	Model      string         `json:"model,omitempty"`
	Message    string         `json:"message,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func New(eventType string) Event {
	return Event{
		Type:      eventType,
		CreatedAt: time.Now(),
	}
}

// WithCorrelation fills stable correlation fields without overwriting values
// that were explicitly set by the event producer.
func (e Event) WithCorrelation(c Correlation) Event {
	if e.RequestID == "" {
		e.RequestID = c.RequestID
	}
	if e.TurnID == "" {
		e.TurnID = c.TurnID
	}
	if e.IntentID == "" {
		e.IntentID = c.IntentID
	}
	if e.DeliveryID == "" {
		e.DeliveryID = c.DeliveryID
	}
	if e.SessionID == "" {
		e.SessionID = c.SessionID
	}
	if e.UserID == 0 {
		e.UserID = c.UserID
	}
	if e.Actor == "" {
		e.Actor = c.Actor
	}
	if e.ToolName == "" {
		e.ToolName = c.ToolName
	}
	if e.ToolName == "" {
		e.ToolName = e.Tool
	}
	if e.Tool == "" {
		e.Tool = e.ToolName
	}
	if e.ToolCallID == "" {
		e.ToolCallID = c.ToolCallID
	}
	if e.Provider == "" {
		e.Provider = c.Provider
	}
	if e.Model == "" {
		e.Model = c.Model
	}
	return e
}
