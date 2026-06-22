package inbound

import (
	"fmt"
	"strings"

	"project-yume/internal/handler"
	"project-yume/internal/utils"
)

type NormalizeStage struct{}

func NewNormalizeStage() *NormalizeStage {
	return &NormalizeStage{}
}

func (s *NormalizeStage) Name() string {
	return "normalize"
}

func (s *NormalizeStage) Process(ctx *handler.MessageContext) error {
	raw := strings.TrimSpace(ctx.RawMessage)
	if raw == "" {
		return skip("empty normalized message")
	}

	if len(ctx.Parts) > 0 {
		textParts := make([]string, 0, len(ctx.Parts))
		imageCount := 0
		ocrParts := make([]string, 0, len(ctx.Parts))
		for _, part := range ctx.Parts {
			switch part.Type {
			case "text":
				text := strings.TrimSpace(part.Text)
				if text != "" {
					textParts = append(textParts, text)
				}
			case "image":
				imageCount++
				ocrText := strings.TrimSpace(part.OCRText)
				if ocrText != "" {
					ocrParts = append(ocrParts, ocrText)
				}
			}
		}

		switch {
		case len(textParts) == 0 && imageCount == 0:
			return skip("empty normalized parts")
		case len(textParts) == 0:
			ctx.Message = fmt.Sprintf("用户发送了%d张图片。", imageCount)
			if len(ocrParts) > 0 {
				ctx.Message += "\n\n图片识别文字：\n" + strings.Join(ocrParts, "\n")
			}
		case imageCount == 0:
			ctx.Message = strings.Join(textParts, "\n")
		default:
			ctx.Message = strings.Join(textParts, "\n") + fmt.Sprintf("\n\n用户还发送了%d张图片。", imageCount)
			if len(ocrParts) > 0 {
				ctx.Message += "\n\n图片识别文字：\n" + strings.Join(ocrParts, "\n")
			}
		}
		return nil
	}

	if utils.IsCQCode(raw) {
		normalized := strings.TrimSpace(utils.ExtractImageURL(raw))
		if normalized != "" {
			ctx.Message = "用户发送了一张图片。"
			return nil
		}
		return skip("unsupported cq message")
	}

	ctx.Message = raw
	return nil
}
