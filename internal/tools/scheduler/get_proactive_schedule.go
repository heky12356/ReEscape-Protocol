package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"project-yume/internal/state"
	"project-yume/internal/tools"
)

type GetProactiveScheduleTool struct{}

func NewGetProactiveScheduleTool() *GetProactiveScheduleTool {
	return &GetProactiveScheduleTool{}
}

func (t *GetProactiveScheduleTool) Name() string {
	return "get_proactive_schedule"
}

func (t *GetProactiveScheduleTool) Description() string {
	return "读取当前会话下一次主动触达时间、手动计划摘要、最后主动触达时间和相关 meta。"
}

func (t *GetProactiveScheduleTool) Schema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *GetProactiveScheduleTool) ReadOnly() bool {
	return true
}

func (t *GetProactiveScheduleTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	schedule := state.GetManager().GetProactiveSchedule(turn.SessionID())
	data := scheduleData(schedule)
	return tools.ToolResult{
		Content: fmt.Sprintf("proactive_schedule=%v", data),
		Data:    data,
	}, nil
}

func scheduleData(schedule state.ProactiveSchedule) map[string]any {
	return map[string]any{
		"session_id":          schedule.SessionID,
		"intent_id":           schedule.IntentID,
		"next_scheduled_at":   formatScheduleTime(schedule.NextScheduledAt),
		"last_proactive_at":   formatScheduleTime(schedule.LastProactiveAt),
		"last_interaction_at": formatScheduleTime(schedule.LastInteractionAt),
		"manual":              schedule.Manual,
		"summary":             schedule.Summary,
		"reason":              schedule.Reason,
		"meta":                schedule.Meta,
		"updated_at":          formatScheduleTime(schedule.UpdatedAt),
		"updated_by":          schedule.UpdatedBy,
	}
}

func formatScheduleTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
