package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"project-yume/internal/state"
	"project-yume/internal/tools"
)

type GetStateTool struct{}

func NewGetStateTool() *GetStateTool {
	return &GetStateTool{}
}

func (t *GetStateTool) Name() string {
	return "get_session_state"
}

func (t *GetStateTool) Description() string {
	return "读取当前会话状态、隐藏对话状态、最近回复时间和主动触达计划。"
}

func (t *GetStateTool) Schema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *GetStateTool) ReadOnly() bool {
	return true
}

func (t *GetStateTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	sm := state.GetManager()
	schedule := sm.GetProactiveSchedule(turn.SessionID())
	data := map[string]any{
		"session_id":                    turn.SessionID(),
		"bot_state":                     int(sm.GetState(turn.SessionID())),
		"dialogue_state":                sm.GetDialogueState(turn.SessionID()),
		"time_since_last_reply":         sm.GetTimeSinceLastReply(turn.SessionID()).String(),
		"time_since_last_interaction":   sm.GetTimeSinceLastInteraction(turn.SessionID()).String(),
		"next_scheduled_at":             formatTime(schedule.NextScheduledAt),
		"last_proactive_at":             formatTime(schedule.LastProactiveAt),
		"proactive_schedule_manual":     schedule.Manual,
		"proactive_schedule_summary":    schedule.Summary,
		"proactive_schedule_reason":     schedule.Reason,
		"proactive_schedule_meta":       schedule.Meta,
		"proactive_schedule_updated_at": formatTime(schedule.UpdatedAt),
		"proactive_schedule_updated_by": schedule.UpdatedBy,
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("session_state=%v", data),
		Data:    data,
	}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
