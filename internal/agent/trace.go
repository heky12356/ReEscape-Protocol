package agent

import (
	"time"

	"project-yume/internal/llm"
)

type Trace struct {
	RequestID string               `json:"request_id"`
	SessionID string               `json:"session_id"`
	Task      string               `json:"task,omitempty"`
	Provider  string               `json:"provider,omitempty"`
	Model     string               `json:"model,omitempty"`
	Failure   *llm.FailureSnapshot `json:"failure,omitempty"`
	Steps     []StepTrace          `json:"steps"`
}

type StepTrace struct {
	Index        int           `json:"index"`
	ToolName     string        `json:"tool_name,omitempty"`
	ToolProvider string        `json:"tool_provider,omitempty"`
	ToolOrigin   string        `json:"tool_origin,omitempty"`
	ToolVersion  string        `json:"tool_version,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
	Arguments    string        `json:"arguments,omitempty"`
	Observation  string        `json:"observation,omitempty"`
	Error        string        `json:"error,omitempty"`
	Denied       bool          `json:"denied,omitempty"`
	Duration     time.Duration `json:"duration"`
	FinishedAt   time.Time     `json:"finished_at"`
}
