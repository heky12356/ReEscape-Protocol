package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domain "project-yume/internal/domain/intent"
	"project-yume/internal/tools"
)

type GetIntentTool struct{}

func NewGetIntentTool() *GetIntentTool { return &GetIntentTool{} }
func (t *GetIntentTool) Name() string  { return "get_intent" }
func (t *GetIntentTool) Description() string {
	return "查询当前用户和会话中尚未完成的后续动作、提醒和主动联系意图。"
}
func (t *GetIntentTool) Schema() tools.Schema {
	return tools.Schema{Type: "object", Properties: map[string]tools.Property{
		"include_completed": {Type: "boolean", Description: "是否包含已完成或已取消的历史意图。"},
	}, AdditionalProperties: false}
}
func (t *GetIntentTool) ReadOnly() bool { return true }

func (t *GetIntentTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	var args struct {
		IncludeCompleted bool `json:"include_completed"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	statuses := []domain.Status{domain.StatusPending, domain.StatusClaimed, domain.StatusDeferred}
	if args.IncludeCompleted {
		statuses = nil
	}
	items := stateIntentStore().List(turn.UserID(), turn.SessionID(), statuses...)
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{"id": item.ID, "kind": item.Kind, "summary": item.Summary, "action": item.Action, "due_at": formatTime(item.DueAt), "status": item.Status, "priority": item.Priority, "open_loop_id": item.OpenLoopID})
	}
	return tools.ToolResult{Content: fmt.Sprintf("intents=%v", data), Data: data}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
