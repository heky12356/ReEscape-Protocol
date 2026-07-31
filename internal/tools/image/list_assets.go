package image

import (
	"context"
	"encoding/json"
	"strings"

	"project-yume/internal/assets"
	"project-yume/internal/tools"
)

type ListAssetsTool struct{}

type listAssetsInput struct {
	Limit int `json:"limit"`
}

func NewListAssetsTool() *ListAssetsTool {
	return &ListAssetsTool{}
}

func (t *ListAssetsTool) Name() string {
	return "list_image_assets"
}

func (t *ListAssetsTool) Description() string {
	return "列出可用图片素材。最终回复需要发图时可使用 [[image:asset_id]] 指令，由 runtime 统一发送。"
}

func (t *ListAssetsTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"limit": {
				Type:        "integer",
				Description: "最多返回多少个素材，默认 8。",
			},
		},
		AdditionalProperties: false,
	}
}

func (t *ListAssetsTool) ReadOnly() bool {
	return true
}

func (t *ListAssetsTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args listAssetsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	limit := args.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	imageAssets, err := assets.ListImageAssets()
	if err != nil {
		return tools.ToolResult{}, err
	}
	filtered := make([]assets.ImageAsset, 0, limit)
	for _, asset := range imageAssets {
		if !asset.Enabled || strings.TrimSpace(asset.ID) == "" {
			continue
		}
		filtered = append(filtered, asset)
		if len(filtered) >= limit {
			break
		}
	}
	if len(filtered) == 0 {
		return tools.ToolResult{Content: "当前没有可用图片素材。"}, nil
	}

	data, _ := json.Marshal(filtered)
	return tools.ToolResult{
		Content: string(data),
		Data:    filtered,
	}, nil
}
