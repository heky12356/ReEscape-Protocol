package time

import (
	"context"
	"encoding/json"
	stdtime "time"

	"project-yume/internal/service"
	"project-yume/internal/tools"
)

type GetCurrentTimeContextTool struct{}

func NewGetCurrentTimeContextTool() *GetCurrentTimeContextTool {
	return &GetCurrentTimeContextTool{}
}

func (t *GetCurrentTimeContextTool) Name() string {
	return "get_current_time_context"
}

func (t *GetCurrentTimeContextTool) Description() string {
	return "读取当前本地时间、时区和相对日期理解规则。"
}

func (t *GetCurrentTimeContextTool) Schema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *GetCurrentTimeContextTool) ReadOnly() bool {
	return true
}

func (t *GetCurrentTimeContextTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	referenceTime := turn.ReferenceTime()
	if referenceTime.IsZero() {
		referenceTime = stdtime.Now()
	}
	content := service.BuildTimeContext(referenceTime)
	if content == "" {
		content = "时间上下文已关闭。"
	}

	return tools.ToolResult{
		Content: content,
		Data: map[string]any{
			"reference_time": referenceTime.Format(stdtime.RFC3339),
		},
	}, nil
}
