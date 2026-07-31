package affection

import (
	"context"
	"encoding/json"
	"fmt"

	domain "project-yume/internal/domain/affection"
	"project-yume/internal/tools"
)

type GetAffectionTool struct{}

func NewGetAffectionTool() *GetAffectionTool {
	return &GetAffectionTool{}
}

func (t *GetAffectionTool) Name() string {
	return "get_affection"
}

func (t *GetAffectionTool) Description() string {
	return "读取当前用户好感度、阶段、最近原因和历史变化。"
}

func (t *GetAffectionTool) Schema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *GetAffectionTool) ReadOnly() bool {
	return true
}

func (t *GetAffectionTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	state := domain.GetManager().Get(turn.UserID())
	return tools.ToolResult{
		Content: fmt.Sprintf("affection=%d stage=%s", state.Score, state.Stage),
		Data:    state,
	}, nil
}
