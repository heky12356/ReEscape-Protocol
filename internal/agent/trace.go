package agent

import "time"

type Trace struct {
	RequestID string      `json:"request_id"`
	SessionID string      `json:"session_id"`
	Steps     []StepTrace `json:"steps"`
}

type StepTrace struct {
	Index       int           `json:"index"`
	ToolName    string        `json:"tool_name,omitempty"`
	ToolCallID  string        `json:"tool_call_id,omitempty"`
	Arguments   string        `json:"arguments,omitempty"`
	Observation string        `json:"observation,omitempty"`
	Error       string        `json:"error,omitempty"`
	Denied      bool          `json:"denied,omitempty"`
	Duration    time.Duration `json:"duration"`
	FinishedAt  time.Time     `json:"finished_at"`
}
