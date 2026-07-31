package tools

import "project-yume/internal/eventlog"

type ToolResult struct {
	Content string           `json:"content"`
	Data    any              `json:"data,omitempty"`
	Events  []eventlog.Event `json:"events,omitempty"`
	Mutated bool             `json:"mutated"`
}

type ExecutionResult struct {
	ToolCallID string
	ToolName   string
	Content    string
	Result     ToolResult
	DurationMs int64
	Error      string
	Denied     bool
}
