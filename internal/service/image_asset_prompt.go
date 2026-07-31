package service

import (
	"fmt"
	"strings"

	"project-yume/internal/assets"
	"project-yume/internal/config"
)

const imageAssetPromptLimit = 4

func BuildImageAssetPromptContext(currentMessage string) string {
	cfg := config.GetConfig()
	if !cfg.EnableImageAssetReply {
		return ""
	}

	candidates, err := assets.ListRelevantImageAssets(currentMessage, imageAssetPromptLimit)
	if err != nil || len(candidates) == 0 {
		return ""
	}

	lines := []string{
		"【图片素材】",
		"如果你判断适合发送现有图片素材，可以在回复中插入 [[image:asset_id]]。",
		"该指令不会展示给用户，系统会把它替换成图片发送。",
		"每次回复最多使用一张图片素材。",
		"当前候选素材：",
	}

	for _, asset := range candidates {
		description := strings.TrimSpace(asset.Description)
		if description == "" {
			description = strings.TrimSpace(asset.Title)
		}
		tags := strings.Join(asset.Tags, "、")
		if tags != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s（标签：%s）", asset.ID, description, tags))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", asset.ID, description))
	}

	return strings.Join(lines, "\n")
}
