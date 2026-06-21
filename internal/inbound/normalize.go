package inbound

import (
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

	if utils.IsCQCode(raw) {
		normalized := strings.TrimSpace(utils.ExtractImageURL(raw))
		if normalized == "" {
			return skip("unsupported cq message")
		}
		ctx.Message = normalized
		return nil
	}

	ctx.Message = raw
	return nil
}
