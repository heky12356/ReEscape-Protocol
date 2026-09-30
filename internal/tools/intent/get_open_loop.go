package intent

import (
	"context"
	"encoding/json"
	"fmt"

	"project-yume/internal/state"
	"project-yume/internal/tools"
)

type GetOpenLoopTool struct{}

func NewGetOpenLoopTool() *GetOpenLoopTool { return &GetOpenLoopTool{} }
func (t *GetOpenLoopTool) Name() string    { return "get_open_loop" }
func (t *GetOpenLoopTool) Description() string {
	return "查询当前会话中仍未闭合的问题、待跟进承诺和未完成话题。"
}
func (t *GetOpenLoopTool) Schema() tools.Schema {
	return tools.Schema{Type: "object", Properties: map[string]tools.Property{"include_closed": {Type: "boolean"}}, AdditionalProperties: false}
}
func (t *GetOpenLoopTool) ReadOnly() bool { return true }
func (t *GetOpenLoopTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	var args struct {
		IncludeClosed bool `json:"include_closed"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	items := state.GetManager().OpenLoopStore().List(turn.UserID(), turn.SessionID(), args.IncludeClosed)
	return tools.ToolResult{Content: fmt.Sprintf("open_loops=%v", items), Data: items}, nil
}
