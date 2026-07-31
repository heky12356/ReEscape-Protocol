package affection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	domain "project-yume/internal/domain/affection"
	"project-yume/internal/eventlog"
	"project-yume/internal/tools"
)

type UpdateAffectionTool struct{}

type updateAffectionInput struct {
	Delta      int      `json:"delta"`
	Reason     string   `json:"reason"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
}

func NewUpdateAffectionTool() *UpdateAffectionTool {
	return &UpdateAffectionTool{}
}

func (t *UpdateAffectionTool) Name() string {
	return "update_affection"
}

func (t *UpdateAffectionTool) Description() string {
	return "按 policy 调整当前用户好感度。写入会被单次和每日上限裁剪。"
}

func (t *UpdateAffectionTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"delta": {
				Type:        "integer",
				Description: "建议变化量，可正可负。",
			},
			"reason": {
				Type:        "string",
				Description: "变化原因。",
			},
			"confidence": {
				Type:        "number",
				Description: "对该判断的置信度，低于阈值时会被拒绝。",
			},
			"tags": {
				Type:        "array",
				Description: "相关标签。",
				Items: &tools.Property{
					Type: "string",
				},
			},
		},
		Required:             []string{"delta", "reason"},
		AdditionalProperties: false,
	}
}

func (t *UpdateAffectionTool) ReadOnly() bool {
	return false
}

func (t *UpdateAffectionTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args updateAffectionInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.Reason = strings.TrimSpace(args.Reason)
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}

	result, err := domain.GetManager().Update(turn.UserID(), domain.UpdateRequest{
		Delta:      args.Delta,
		Reason:     args.Reason,
		Confidence: args.Confidence,
		Tags:       args.Tags,
	})
	if err != nil {
		return tools.ToolResult{}, err
	}

	event := eventlog.Event{
		Type:      "affection_updated",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      t.Name(),
		Message:   result.State.LastReason,
		Data: map[string]any{
			"requested_delta": result.RequestedDelta,
			"applied_delta":   result.AppliedDelta,
			"policy":          result.Policy,
			"score":           result.State.Score,
			"stage":           result.State.Stage,
		},
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("applied_delta=%d score=%d stage=%s", result.AppliedDelta, result.State.Score, result.State.Stage),
		Data:    result,
		Events:  []eventlog.Event{event},
		Mutated: true,
	}, nil
}
