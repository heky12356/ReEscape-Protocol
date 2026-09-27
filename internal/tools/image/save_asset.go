package image

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"project-yume/internal/assets"
	"project-yume/internal/eventlog"
	"project-yume/internal/model"
	"project-yume/internal/tools"
)

type SaveAssetTool struct{}

type saveAssetInput struct {
	AssetID     string   `json:"asset_id"`
	SourceURL   string   `json:"source_url"`
	ImageIndex  int      `json:"image_index"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Overwrite   bool     `json:"overwrite"`
}

type partsTurnView interface {
	Parts() []model.MessagePart
}

func NewSaveAssetTool() *SaveAssetTool {
	return &SaveAssetTool{}
}

func (t *SaveAssetTool) Name() string {
	return "save_image_asset"
}

func (t *SaveAssetTool) Description() string {
	return "将当前消息中的图片保存到图片素材库，或保存一个用户明确提供的公开图片 URL。保存后可使用 [[image:asset_id]] 发送。仅在用户明确要求收藏、保存或记住图片时调用。"
}

func (t *SaveAssetTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"asset_id": {
				Type:        "string",
				Description: "素材唯一 ID，只能使用字母、数字、点、短横线和下划线。",
			},
			"source_url": {
				Type:        "string",
				Description: "可选的公开 http/https 图片 URL；省略时使用当前消息中的图片。",
			},
			"image_index": {
				Type:        "integer",
				Description: "当前消息包含多张图片时使用的图片下标，从 0 开始。",
			},
			"title": {
				Type:        "string",
				Description: "图片素材标题，省略时使用 asset_id。",
			},
			"description": {
				Type:        "string",
				Description: "图片适用场景的简短描述。",
			},
			"tags": {
				Type:        "array",
				Description: "用于后续匹配图片的标签。",
				Items:       &tools.Property{Type: "string"},
			},
			"overwrite": {
				Type:        "boolean",
				Description: "是否覆盖已有同 ID 素材；只有用户明确要求更新时才设为 true。",
			},
		},
		Required:             []string{"asset_id"},
		AdditionalProperties: false,
	}
}

func (t *SaveAssetTool) ReadOnly() bool {
	return false
}

func (t *SaveAssetTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	if turn == nil {
		return tools.ToolResult{}, fmt.Errorf("turn is required")
	}

	var args saveAssetInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.AssetID = strings.TrimSpace(args.AssetID)
	args.SourceURL = strings.TrimSpace(args.SourceURL)
	args.Title = strings.TrimSpace(args.Title)
	args.Description = strings.TrimSpace(args.Description)

	sourceURL := args.SourceURL
	if sourceURL == "" {
		partsView, ok := turn.(partsTurnView)
		if !ok {
			return tools.ToolResult{}, fmt.Errorf("current turn does not expose image parts; source_url is required")
		}
		imageParts := make([]model.MessagePart, 0)
		for _, part := range partsView.Parts() {
			if part.Type == "image" {
				imageParts = append(imageParts, part)
			}
		}
		if args.ImageIndex < 0 || args.ImageIndex >= len(imageParts) {
			return tools.ToolResult{}, fmt.Errorf("image_index %d is unavailable; current turn has %d images", args.ImageIndex, len(imageParts))
		}
		sourceURL = strings.TrimSpace(imageParts[args.ImageIndex].URL)
		if sourceURL == "" {
			return tools.ToolResult{}, fmt.Errorf("current image has no accessible URL; source_url is required")
		}
	}

	result, err := assets.SaveImageAsset(ctx, assets.SaveImageAssetRequest{
		SourceURL:   sourceURL,
		ID:          args.AssetID,
		Title:       args.Title,
		Description: args.Description,
		Tags:        args.Tags,
		Overwrite:   args.Overwrite,
	})
	if err != nil {
		return tools.ToolResult{}, err
	}

	event := eventlog.Event{
		Type:      "image_asset_saved",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      t.Name(),
		Message:   result.Asset.Title,
		Data: map[string]any{
			"asset_id":     result.Asset.ID,
			"file":         result.Asset.File,
			"content_type": result.ContentType,
			"bytes":        result.Bytes,
		},
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("图片素材已保存：%s。之后可使用 [[image:%s]] 发送。", result.Asset.Title, result.Asset.ID),
		Data:    result,
		Events:  []eventlog.Event{event},
		Mutated: true,
	}, nil
}
