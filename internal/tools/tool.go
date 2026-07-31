package tools

import (
	"context"
	"encoding/json"
	"time"
)

type TurnView interface {
	RequestID() string
	SessionID() string
	UserID() int64
	GroupID() int64
	ChatType() int
	Message() string
	ReferenceTime() time.Time
	Trigger() string
	Actor() string
}

type Tool interface {
	Name() string
	Description() string
	Schema() Schema
	ReadOnly() bool
	Execute(ctx context.Context, turn TurnView, input json.RawMessage) (ToolResult, error)
}
